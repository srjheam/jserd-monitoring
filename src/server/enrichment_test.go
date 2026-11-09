package main

import (
	"server/internal/config"
	"server/internal/editorial"
	"server/internal/ojs"
	"testing"
	"time"
)

func TestConfigureLiveEnrichmentSkipsWiringInFixturesMode(t *testing.T) {
	coordinator := &editorial.Coordinator{}
	configureLiveEnrichment(coordinator, config.Config{SourceMode: config.SourceModeFixtures}, nil, editorial.Evaluator{})

	if coordinator.OJSClient != nil || coordinator.Enrichers != nil || coordinator.PreProcessEnrichers != nil {
		t.Fatal("fixtures mode must not wire any live enrichment stage")
	}
}

func TestConfigureLiveEnrichmentWiresStagesInLiveMode(t *testing.T) {
	client, err := ojs.NewClient("https://example.test", "jserd", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	coordinator := &editorial.Coordinator{}
	cfg := config.Config{SourceMode: config.SourceModeLive, Username: "editor", Password: "secret"}

	configureLiveEnrichment(coordinator, cfg, client, editorial.Evaluator{})

	if coordinator.OJSClient == nil || coordinator.OJSUsername != cfg.Username || coordinator.OJSPassword != cfg.Password {
		t.Fatalf("live mode must wire OJS credentials: %#v", coordinator)
	}
	if len(coordinator.Enrichers) != 3 {
		t.Fatalf("Enrichers=%d", len(coordinator.Enrichers))
	}
	if len(coordinator.PreProcessEnrichers) != 4 {
		t.Fatalf("PreProcessEnrichers=%d", len(coordinator.PreProcessEnrichers))
	}
}
