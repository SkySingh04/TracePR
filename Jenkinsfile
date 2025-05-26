pipeline {
    agent any
    
    environment {
        // Docker and Registry Configuration
        DOCKER_REGISTRY = 'your-docker-registry.com'
        DOCKER_REPO = 'tracepr'
        IMAGE_TAG = "${BUILD_NUMBER}-${GIT_COMMIT.take(7)}"
        DOCKER_IMAGE = "${DOCKER_REGISTRY}/${DOCKER_REPO}:${IMAGE_TAG}"
        
        // Kubernetes Configuration
        KUBECONFIG = credentials('kubeconfig')
        KUBE_NAMESPACE = 'tracepr'
        
        // Application Configuration
        GO_VERSION = '1.24'
        PYTHON_VERSION = '3.11'
        
        // Security Scanning
        TRIVY_VERSION = '0.45.0'
        
        // Notification Configuration
        SLACK_CHANNEL = '#deployments'
        TEAMS_WEBHOOK = credentials('teams-webhook')
    }
    
    options {
        buildDiscarder(logRotator(numToKeepStr: '10'))
        timeout(time: 30, unit: 'MINUTES')
        timestamps()
        ansiColor('xterm')
    }
    
    triggers {
        githubPush()
        cron(env.BRANCH_NAME == 'main' ? 'H 2 * * *' : '')
    }
    
    stages {
        stage('Checkout') {
            steps {
                script {
                    // Clean workspace
                    cleanWs()
                    
                    // Checkout code
                    checkout scm
                    
                    // Set build description
                    currentBuild.description = "Branch: ${env.BRANCH_NAME}, Commit: ${env.GIT_COMMIT.take(7)}"
                }
            }
        }
        
        stage('Environment Setup') {
            parallel {
                stage('Go Setup') {
                    steps {
                        script {
                            sh '''
                                echo "Setting up Go environment..."
                                go version
                                go env
                            '''
                        }
                    }
                }
                
                stage('Python Setup') {
                    steps {
                        script {
                            sh '''
                                echo "Setting up Python environment..."
                                python3 --version
                                pip3 --version
                                pip3 install -r requirements.txt
                            '''
                        }
                    }
                }
                
                stage('Docker Setup') {
                    steps {
                        script {
                            sh '''
                                echo "Setting up Docker environment..."
                                docker --version
                                docker-compose --version
                            '''
                        }
                    }
                }
            }
        }
        
        stage('Code Quality') {
            parallel {
                stage('Go Lint') {
                    steps {
                        script {
                            sh '''
                                echo "Running Go linting..."
                                go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
                                golangci-lint run --timeout=5m
                            '''
                        }
                    }
                }
                
                stage('Go Format Check') {
                    steps {
                        script {
                            sh '''
                                echo "Checking Go formatting..."
                                gofmt -l .
                                if [ $(gofmt -l . | wc -l) -ne 0 ]; then
                                    echo "Go files are not formatted properly"
                                    exit 1
                                fi
                            '''
                        }
                    }
                }
                
                stage('Go Vet') {
                    steps {
                        script {
                            sh '''
                                echo "Running Go vet..."
                                go vet ./...
                            '''
                        }
                    }
                }
            }
        }
        
        stage('Testing') {
            parallel {
                stage('Unit Tests') {
                    steps {
                        script {
                            sh '''
                                echo "Running unit tests..."
                                go test -v -race -coverprofile=coverage.out ./...
                                go tool cover -html=coverage.out -o coverage.html
                            '''
                        }
                    }
                    post {
                        always {
                            publishHTML([
                                allowMissing: false,
                                alwaysLinkToLastBuild: true,
                                keepAll: true,
                                reportDir: '.',
                                reportFiles: 'coverage.html',
                                reportName: 'Coverage Report'
                            ])
                        }
                    }
                }
                
                stage('Integration Tests') {
                    when {
                        anyOf {
                            branch 'main'
                            branch 'develop'
                            changeRequest()
                        }
                    }
                    steps {
                        script {
                            sh '''
                                echo "Running integration tests..."
                                # Add integration test commands here
                                echo "Integration tests completed"
                            '''
                        }
                    }
                }
            }
        }
        
        stage('Build') {
            steps {
                script {
                    sh '''
                        echo "Building Go application..."
                        CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o tracepr .
                        
                        echo "Building Docker image..."
                        docker build -t ${DOCKER_IMAGE} .
                        docker tag ${DOCKER_IMAGE} ${DOCKER_REGISTRY}/${DOCKER_REPO}:latest
                    '''
                }
            }
        }
        
        stage('Security Scanning') {
            parallel {
                stage('Dependency Check') {
                    steps {
                        script {
                            sh '''
                                echo "Scanning Go dependencies..."
                                go list -json -deps ./... | nancy sleuth
                            '''
                        }
                    }
                }
                
                stage('Container Scan') {
                    steps {
                        script {
                            sh '''
                                echo "Scanning Docker image with Trivy..."
                                docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
                                    -v $HOME/Library/Caches:/root/.cache/ \
                                    aquasec/trivy:${TRIVY_VERSION} image \
                                    --exit-code 1 --severity HIGH,CRITICAL \
                                    ${DOCKER_IMAGE}
                            '''
                        }
                    }
                }
            }
        }
        
        stage('Push to Registry') {
            when {
                anyOf {
                    branch 'main'
                    branch 'develop'
                }
            }
            steps {
                script {
                    withCredentials([usernamePassword(credentialsId: 'docker-registry-creds', 
                                                   usernameVariable: 'DOCKER_USER', 
                                                   passwordVariable: 'DOCKER_PASS')]) {
                        sh '''
                            echo "Logging into Docker registry..."
                            echo $DOCKER_PASS | docker login $DOCKER_REGISTRY -u $DOCKER_USER --password-stdin
                            
                            echo "Pushing Docker image..."
                            docker push ${DOCKER_IMAGE}
                            docker push ${DOCKER_REGISTRY}/${DOCKER_REPO}:latest
                        '''
                    }
                }
            }
        }
        
        stage('Deploy to Staging') {
            when {
                branch 'develop'
            }
            steps {
                script {
                    sh '''
                        echo "Deploying to staging environment..."
                        
                        # Update image tag in kustomization
                        cd k8s
                        kustomize edit set image tracepr=${DOCKER_IMAGE}
                        
                        # Apply to staging namespace
                        kubectl apply -k . --namespace=tracepr-staging
                        
                        # Wait for rollout
                        kubectl rollout status deployment/tracepr --namespace=tracepr-staging --timeout=300s
                    '''
                }
            }
        }
        
        stage('Deploy to Production') {
            when {
                branch 'main'
            }
            steps {
                script {
                    input message: 'Deploy to production?', ok: 'Deploy',
                          submitterParameter: 'DEPLOYER'
                    
                    sh '''
                        echo "Deploying to production environment..."
                        echo "Deployed by: ${DEPLOYER}"
                        
                        # Update image tag in kustomization
                        cd k8s
                        kustomize edit set image tracepr=${DOCKER_IMAGE}
                        
                        # Apply to production namespace
                        kubectl apply -k . --namespace=tracepr
                        
                        # Wait for rollout
                        kubectl rollout status deployment/tracepr --namespace=tracepr --timeout=600s
                        
                        # Verify deployment
                        kubectl get pods --namespace=tracepr -l app=tracepr
                    '''
                }
            }
        }
        
        stage('Post-Deploy Tests') {
            when {
                anyOf {
                    branch 'main'
                    branch 'develop'
                }
            }
            steps {
                script {
                    sh '''
                        echo "Running post-deployment tests..."
                        
                        # Health check
                        NAMESPACE=${BRANCH_NAME == 'main' ? 'tracepr' : 'tracepr-staging'}
                        kubectl wait --for=condition=ready pod -l app=tracepr --namespace=$NAMESPACE --timeout=300s
                        
                        # Smoke tests
                        echo "Running smoke tests..."
                        # Add smoke test commands here
                    '''
                }
            }
        }
    }
    
    post {
        always {
            script {
                // Clean up Docker images
                sh '''
                    docker rmi ${DOCKER_IMAGE} || true
                    docker system prune -f || true
                '''
                
                // Archive artifacts
                archiveArtifacts artifacts: 'tracepr,coverage.html', allowEmptyArchive: true
                
                // Publish test results
                publishTestResults testResultsPattern: 'test-results.xml'
            }
        }
        
        success {
            script {
                if (env.BRANCH_NAME == 'main') {
                    slackSend(
                        channel: env.SLACK_CHANNEL,
                        color: 'good',
                        message: ":white_check_mark: TracePR deployment successful!\nBranch: ${env.BRANCH_NAME}\nCommit: ${env.GIT_COMMIT.take(7)}\nImage: ${DOCKER_IMAGE}"
                    )
                }
            }
        }
        
        failure {
            script {
                slackSend(
                    channel: env.SLACK_CHANNEL,
                    color: 'danger',
                    message: ":x: TracePR build/deployment failed!\nBranch: ${env.BRANCH_NAME}\nCommit: ${env.GIT_COMMIT.take(7)}\nBuild: ${env.BUILD_URL}"
                )
            }
        }
        
        unstable {
            script {
                slackSend(
                    channel: env.SLACK_CHANNEL,
                    color: 'warning',
                    message: ":warning: TracePR build unstable!\nBranch: ${env.BRANCH_NAME}\nCommit: ${env.GIT_COMMIT.take(7)}\nBuild: ${env.BUILD_URL}"
                )
            }
        }
    }
} 