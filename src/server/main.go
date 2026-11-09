package main

import (
	"context"
	"log"
	"server/internal/audit"
	"server/internal/collector"
	"server/internal/config"
	"server/internal/editorial"
	"server/internal/monitoring"
	"server/internal/notifications"
	"server/internal/ojs"
	"server/internal/state"
	"time"
	_ "time/tzdata"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"
	"github.com/pocketbase/pocketbase/tools/cron"

	mail "server/internal/email"
	_ "server/migrations"
)

func main() {
	requireEnvFile()
	app := newApp()
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}
	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{})
	registerImmutableHooksOnServe(app)
	registerBackupPolicy(app)
	monitoring.Register(app)
	registerSMTPSettings(app, cfg)

	transport := newTransport(cfg)
	worker := newWorker(app, cfg, transport)
	store := state.NewStore(app)
	notificationService := notifications.NewService(app)
	location, _ := time.LoadLocation(cfg.EditorialTimezone)

	coordinator, err := buildEditorialCoordinator(app, cfg, transport, store, notificationService, location)
	if err != nil {
		log.Fatalf("configuração OJS inválida: %v", err)
	}

	worker.EditorTargets = notificationService.EditorTargets

	registerCollectionJob(app.Cron(), cfg.OJSCollectionCron, coordinator)
	app.Cron().MustAdd("email-delivery-worker", cfg.EmailWorkerCron, newRunDue(app, worker))
	app.Cron().SetTimezone(location)
	registerManualCollectionHooks(app, coordinator)
	app.OnServe().BindFunc(serveStaticFiles)
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

// buildEditorialCoordinator wires the editorial pipeline: the OJS chief
// editor source, delivery, evaluation, pruning, and live enrichment, all
// sharing the same OJS client and config.
func buildEditorialCoordinator(app core.App, cfg config.Config, transport mail.Transport, store *state.Store, notificationService *notifications.Service, location *time.Location) (*editorial.Coordinator, error) {
	collectPipeline := func(ctx context.Context) (collector.CycleResult, error) {
		return (collector.Collector{Config: cfg}).Run(ctx)
	}
	schedule, _ := cron.NewSchedule(cfg.SummaryCron)
	ojsClient, err := ojs.NewClient(cfg.BaseURL, cfg.JournalContext, cfg.RequestTimeout)
	if err != nil {
		return nil, err
	}
	roleSource := notifications.OJSEditorTargets{}
	if cfg.SourceMode == config.SourceModeLive {
		roleSource = notifications.OJSEditorTargets{Client: ojsClient, Username: cfg.Username, Password: cfg.Password}
	}
	notificationService.EditorTargets = roleSource.Keys
	chiefSource := editorial.OJSChiefEditorSource{Client: ojsClient, Username: cfg.Username, Password: cfg.Password, GroupID: cfg.OJSChiefEditorGroupID, GroupName: cfg.OJSChiefEditorGroupName}
	editorialDelivery := &editorial.DeliveryService{App: app, Resolver: editorial.RecipientResolver{App: app, OJS: chiefSource, GroupID: cfg.OJSChiefEditorGroupID, GroupName: cfg.OJSChiefEditorGroupName}, Renderer: editorial.Renderer{}, Transport: transport, From: cfg.SMTPFromAddress}
	scheduleStore := editorial.ScheduleStore{App: app}
	evaluator := editorial.Evaluator{RN11FallbackDays: cfg.EditorialRN11FallbackDays}
	coordinator := &editorial.Coordinator{App: app, Store: editorial.NewStore(app), Delivery: editorialDelivery, Evaluator: evaluator, Collect: collectPipeline, Process: store.ProcessCycle, CreatePending: notificationService.CreatePending, OnFindingsReady: monitoring.OnFindingsReady, Schedule: schedule, Location: location, NextSummaryDueAt: scheduleStore.NextSummaryDueAt, SetNextSummaryDueAt: scheduleStore.SetNextSummaryDueAt}
	coordinator.Prune = newPruner(store, audit.ArtifactStore{Root: cfg.ArtifactRoot})
	configureLiveEnrichment(coordinator, cfg, ojsClient, evaluator)
	return coordinator, nil
}
