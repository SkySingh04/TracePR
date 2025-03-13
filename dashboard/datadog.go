package dashboard

import (
	"PRism/config"
	"context"
	"encoding/json"
	"fmt"
	"log"

	datadog "github.com/DataDog/datadog-api-client-go/api/v1/datadog"
)

func CreateDatadogDashboard(suggestion config.DashboardSuggestion, cfg config.Config) error {
	log.Printf("Creating Datadog dashboard: %s", suggestion.Name)

	// Debug: Print JSON structure
	log.Printf("Queries JSON: %s", suggestion.Queries)
	log.Printf("Panels JSON: %s", suggestion.Panels)

	// Initialize Datadog client with the v1 API client
	configuration := datadog.NewConfiguration()
	configuration.Host = "api.ap1.datadoghq.com"
	configuration.AddDefaultHeader("DD-API-KEY", cfg.DatadogAPIKey)
	configuration.AddDefaultHeader("DD-APPLICATION-KEY", cfg.DatadogAppKey)
	apiClient := datadog.NewAPIClient(configuration)

	// Parse the queries, panels, and alerts
	var queries []map[string]interface{}
	var panels []map[string]interface{}
	var alerts []map[string]interface{}

	if err := json.Unmarshal([]byte(suggestion.Queries), &queries); err != nil {
		return fmt.Errorf("error parsing queries JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(suggestion.Panels), &panels); err != nil {
		return fmt.Errorf("error parsing panels JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(suggestion.Alerts), &alerts); err != nil {
		return fmt.Errorf("error parsing alerts JSON: %v", err)
	}

	// Create widgets from panels
	widgets := []datadog.Widget{}
	for _, panel := range panels {
		// Debug panel structure
		panelBytes, _ := json.MarshalIndent(panel, "", "  ")
		log.Printf("Processing panel: %s", string(panelBytes))

		title, ok := panel["title"].(string)
		if !ok {
			log.Printf("Warning: panel title is not a string, skipping panel")
			continue
		}

		gridPos, ok := panel["gridPos"].(map[string]interface{})
		if !ok {
			log.Printf("Warning: gridPos is not a map, skipping panel %s", title)
			continue
		}

		// Create a single query based on panel name if targets aren't properly formatted
		queryStr := fmt.Sprintf("avg:system.cpu.user{*} by {host}.rollup(avg, 30)")

		// Handle the targets differently depending on type
		var targets []interface{}

		// Check if targets exists
		targetsVal, targetsExist := panel["targets"]
		if !targetsExist {
			log.Printf("Warning: panel %s has no targets, using default query", title)
		} else {
			// Try to convert to expected format
			switch v := targetsVal.(type) {
			case []interface{}:
				targets = v
			case map[string]interface{}:
				// Convert map to array with single element
				targets = []interface{}{v}
			case string:
				log.Printf("Warning: targets is a string in panel %s, using panel title as query", title)
				queryStr = fmt.Sprintf("avg:system.load.1{$env} by {host}.rollup(avg, 60) as \"%s\"", title)
			default:
				log.Printf("Warning: targets has unexpected type %T in panel %s, using default query", targetsVal, title)
			}
		}

		// Create widget requests
		requests := []datadog.TimeseriesWidgetRequest{}

		// Try to extract query from targets if they exist
		targetsProcessed := false
		for _, target := range targets {
			targetMap, ok := target.(map[string]interface{})
			if !ok {
				log.Printf("Warning: target type %T is not a map in panel %s, skipping target", target, title)
				continue
			}

			// Debug target structure
			targetBytes, _ := json.Marshal(targetMap)
			log.Printf("Target in panel %s: %s", title, string(targetBytes))

			refId, ok := targetMap["refId"].(string)
			if !ok {
				log.Printf("Warning: refId is not a string in panel %s, using default query", title)
				continue
			}

			// Find matching query
			queryFound := false
			for _, query := range queries {
				queryRefId, ok := query["refId"].(string)
				if !ok || queryRefId != refId {
					continue
				}

				queryExpr, ok := query["expr"].(string)
				if !ok {
					log.Printf("Warning: expr is not a string in query %s, skipping query", queryRefId)
					continue
				}

				queryStr = queryExpr
				queryFound = true
				targetsProcessed = true
				break
			}

			if !queryFound {
				log.Printf("Warning: no matching query found for refId %s in panel %s", refId, title)
			}
		}

		// If no targets were successfully processed, use the default query
		if !targetsProcessed {
			log.Printf("Using default query for panel %s: %s", title, queryStr)
		}

		// Create a timeserieswidgetrequest
		request := datadog.TimeseriesWidgetRequest{
			Q:           &queryStr,
			DisplayType: datadog.WIDGETDISPLAYTYPE_LINE.Ptr(),
			Style: &datadog.WidgetRequestStyle{
				Palette:   (*string)(datadog.WIDGETPALETTE_BLACK_ON_LIGHT_GREEN.Ptr()),
				LineType:  datadog.WIDGETLINETYPE_SOLID.Ptr(),
				LineWidth: datadog.WIDGETLINEWIDTH_NORMAL.Ptr(),
			},
		}
		requests = append(requests, request)

		// Extract layout parameters with type checking
		x, ok := getInt64FromFloat(gridPos, "x")
		if !ok {
			log.Printf("Warning: x is not a number in panel %s, using default 0", title)
			x = 0
		}

		y, ok := getInt64FromFloat(gridPos, "y")
		if !ok {
			log.Printf("Warning: y is not a number in panel %s, using default 0", title)
			y = 0
		}

		w, ok := getInt64FromFloat(gridPos, "w")
		if !ok {
			log.Printf("Warning: w is not a number in panel %s, using default 12", title)
			w = 12
		}

		h, ok := getInt64FromFloat(gridPos, "h")
		if !ok {
			log.Printf("Warning: h is not a number in panel %s, using default 8", title)
			h = 8
		}

		// Create widget definition
		legendSize := "small"
		timeseriesDef := datadog.NewTimeseriesWidgetDefinitionWithDefaults()
		timeseriesDef.SetRequests(requests)
		timeseriesDef.SetTitle(title)
		timeseriesDef.SetLegendSize(legendSize)

		// Create widget with definition and layout
		widget := datadog.Widget{
			Definition: datadog.WidgetDefinition{
				TimeseriesWidgetDefinition: timeseriesDef,
			},
			Layout: &datadog.WidgetLayout{
				X:      x,
				Y:      y,
				Width:  w,
				Height: h,
			},
		}
		widgets = append(widgets, widget)
	}

	// Ensure we have at least one widget
	if len(widgets) == 0 {
		log.Printf("Warning: No valid widgets created, adding a default widget")
		defaultQuery := "avg:system.cpu.user{*}"
		defaultRequest := datadog.TimeseriesWidgetRequest{
			Q:           &defaultQuery,
			DisplayType: datadog.WIDGETDISPLAYTYPE_LINE.Ptr(),
		}

		defaultDef := datadog.NewTimeseriesWidgetDefinitionWithDefaults()
		defaultDef.SetRequests([]datadog.TimeseriesWidgetRequest{defaultRequest})
		defaultDef.SetTitle("Default Widget")

		defaultWidget := datadog.Widget{
			Definition: datadog.WidgetDefinition{
				TimeseriesWidgetDefinition: defaultDef,
			},
			Layout: &datadog.WidgetLayout{
				X:      int64(0),
				Y:      int64(0),
				Width:  int64(12),
				Height: int64(8),
			},
		}
		widgets = append(widgets, defaultWidget)
	}

	// Create dashboard template variables
	defaultVal := "*"
	prefix := "env"
	name := "env"
	templateVar := datadog.DashboardTemplateVariable{
		Name:    name,
		Prefix:  *datadog.NewNullableString(&prefix),
		Default: *datadog.NewNullableString(&defaultVal),
	}

	// Create dashboard request
	dashTitle := suggestion.Name
	dashDesc := "Created by PRism"
	layoutType := datadog.DASHBOARDLAYOUTTYPE_ORDERED
	dashboardRequest := datadog.Dashboard{
		Title:             dashTitle,
		Description:       *datadog.NewNullableString(&dashDesc),
		LayoutType:        layoutType,
		Widgets:           widgets,
		TemplateVariables: []datadog.DashboardTemplateVariable{templateVar},
		NotifyList:        []string{},
	}

	// Debug the final request
	requestBytes, _ := json.MarshalIndent(dashboardRequest, "", "  ")
	log.Printf("Dashboard request: %s", string(requestBytes))

	// Create the dashboard
	ctx := context.Background()
	dashboard, resp, err := apiClient.DashboardsApi.CreateDashboard(ctx, dashboardRequest)
	if err != nil {
		log.Printf("Failed to create Datadog dashboard, status: %v", resp.StatusCode)
		if resp != nil && resp.Body != nil {
			body := make([]byte, 1024)
			n, _ := resp.Body.Read(body)
			log.Printf("Error response: %s", string(body[:n]))
		}
		return fmt.Errorf("failed to create Datadog dashboard: %w", err)
	}

	log.Printf("Successfully created Datadog dashboard with ID: %s", dashboard.GetId())
	return nil
}

// Helper function to safely convert interface{} to int64
func getInt64FromFloat(m map[string]interface{}, key string) (int64, bool) {
	val, exists := m[key]
	if !exists {
		return 0, false
	}

	// Try as float64 (common for JSON numbers)
	if floatVal, ok := val.(float64); ok {
		return int64(floatVal), true
	}

	// Try as int
	if intVal, ok := val.(int); ok {
		return int64(intVal), true
	}

	// Try as int64
	if int64Val, ok := val.(int64); ok {
		return int64Val, true
	}

	// Try as string
	if strVal, ok := val.(string); ok {
		var result float64
		if _, err := fmt.Sscanf(strVal, "%f", &result); err == nil {
			return int64(result), true
		}
	}

	return 0, false
}
