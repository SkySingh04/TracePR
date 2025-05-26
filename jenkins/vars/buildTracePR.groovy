#!/usr/bin/env groovy

def call(Map config) {
    pipeline {
        agent any
        
        environment {
            DOCKER_REGISTRY = config.dockerRegistry ?: 'your-docker-registry.com'
            DOCKER_REPO = config.dockerRepo ?: 'tracepr'
            IMAGE_TAG = config.imageTag ?: "${BUILD_NUMBER}-${GIT_COMMIT.take(7)}"
            DOCKER_IMAGE = "${DOCKER_REGISTRY}/${DOCKER_REPO}:${IMAGE_TAG}"
            KUBE_NAMESPACE = config.kubeNamespace ?: 'tracepr'
            GO_VERSION = config.goVersion ?: '1.24'
            PYTHON_VERSION = config.pythonVersion ?: '3.11'
        }
        
        stages {
            stage('Setup') {
                steps {
                    setupEnvironment(config)
                }
            }
            
            stage('Code Quality') {
                parallel {
                    stage('Lint') {
                        steps {
                            runLinting(config)
                        }
                    }
                    stage('Security') {
                        steps {
                            runSecurityChecks(config)
                        }
                    }
                }
            }
            
            stage('Test') {
                steps {
                    runTests(config)
                }
            }
            
            stage('Build') {
                steps {
                    buildApplication(config)
                }
            }
            
            stage('Deploy') {
                when {
                    anyOf {
                        branch 'main'
                        branch 'develop'
                    }
                }
                steps {
                    deployApplication(config)
                }
            }
        }
        
        post {
            always {
                cleanupResources(config)
            }
            success {
                notifySuccess(config)
            }
            failure {
                notifyFailure(config)
            }
        }
    }
}

def setupEnvironment(Map config) {
    sh '''
        echo "Setting up build environment..."
        go version
        python3 --version
        docker --version
        kubectl version --client
    '''
}

def runLinting(Map config) {
    sh '''
        echo "Running Go linting..."
        go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
        golangci-lint run --timeout=5m
        
        echo "Checking Go formatting..."
        gofmt -l .
        if [ $(gofmt -l . | wc -l) -ne 0 ]; then
            echo "Go files are not formatted properly"
            exit 1
        fi
        
        echo "Running Go vet..."
        go vet ./...
    '''
}

def runSecurityChecks(Map config) {
    sh '''
        echo "Running security checks..."
        go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
        gosec -fmt json -out gosec-report.json ./... || true
        
        echo "Scanning dependencies..."
        go list -json -deps ./... > deps.json
    '''
    
    archiveArtifacts artifacts: 'gosec-report.json,deps.json', allowEmptyArchive: true
}

def runTests(Map config) {
    sh '''
        echo "Running unit tests..."
        go test -v -race -coverprofile=coverage.out -covermode=atomic ./...
        go tool cover -html=coverage.out -o coverage.html
        
        echo "Running benchmark tests..."
        go test -bench=. -benchmem ./... > benchmark-results.txt || true
    '''
    
    publishHTML([
        allowMissing: false,
        alwaysLinkToLastBuild: true,
        keepAll: true,
        reportDir: '.',
        reportFiles: 'coverage.html',
        reportName: 'Coverage Report'
    ])
    
    archiveArtifacts artifacts: 'benchmark-results.txt', allowEmptyArchive: true
}

def buildApplication(Map config) {
    sh '''
        echo "Building Go application..."
        CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags="-w -s" -o tracepr .
        
        echo "Building Docker image..."
        docker build -t ${DOCKER_IMAGE} .
        docker tag ${DOCKER_IMAGE} ${DOCKER_REGISTRY}/${DOCKER_REPO}:latest
    '''
    
    if (config.pushImage != false) {
        withCredentials([usernamePassword(credentialsId: 'docker-registry-creds', 
                                        usernameVariable: 'DOCKER_USER', 
                                        passwordVariable: 'DOCKER_PASS')]) {
            sh '''
                echo "Pushing Docker image..."
                echo $DOCKER_PASS | docker login $DOCKER_REGISTRY -u $DOCKER_USER --password-stdin
                docker push ${DOCKER_IMAGE}
                docker push ${DOCKER_REGISTRY}/${DOCKER_REPO}:latest
            '''
        }
    }
}

def deployApplication(Map config) {
    if (config.deploy != false) {
        sh '''
            echo "Deploying application..."
            cd k8s
            kustomize edit set image tracepr=${DOCKER_IMAGE}
            kubectl apply -k . --namespace=${KUBE_NAMESPACE}
            kubectl rollout status deployment/tracepr --namespace=${KUBE_NAMESPACE} --timeout=600s
        '''
    }
}

def cleanupResources(Map config) {
    sh '''
        echo "Cleaning up resources..."
        docker rmi ${DOCKER_IMAGE} || true
        docker system prune -f || true
    '''
    
    archiveArtifacts artifacts: 'tracepr,coverage.html', allowEmptyArchive: true
}

def notifySuccess(Map config) {
    if (config.notifications?.slack) {
        slackSend(
            channel: config.notifications.slack.channel ?: '#deployments',
            color: 'good',
            message: ":white_check_mark: TracePR build successful!\nBranch: ${env.BRANCH_NAME}\nCommit: ${env.GIT_COMMIT.take(7)}\nImage: ${env.DOCKER_IMAGE}"
        )
    }
}

def notifyFailure(Map config) {
    if (config.notifications?.slack) {
        slackSend(
            channel: config.notifications.slack.channel ?: '#deployments',
            color: 'danger',
            message: ":x: TracePR build failed!\nBranch: ${env.BRANCH_NAME}\nCommit: ${env.GIT_COMMIT.take(7)}\nBuild: ${env.BUILD_URL}"
        )
    }
} 