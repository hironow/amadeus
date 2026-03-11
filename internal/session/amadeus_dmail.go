package session

import (
	"context"
	"fmt"
	"time"

	"github.com/hironow/amadeus/internal/domain"
	"github.com/hironow/amadeus/internal/platform"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// consumeInbox runs Phase 0: scans the inbox for inbound D-Mails and emits
// inbox-consumed events. Returns nil when there are no D-Mails to consume.
func (a *Amadeus) consumeInbox(ctx context.Context, quiet bool) error {
	span := trace.SpanFromContext(ctx)

	consumed, scanErr := a.Store.ScanInbox(ctx)
	if scanErr != nil {
		return fmt.Errorf("scan inbox: %w", scanErr)
	}
	if len(consumed) == 0 {
		return nil
	}
	if !quiet {
		a.Logger.Info("Consumed %d report(s) from inbox", len(consumed))
	}
	span.AddEvent("inbox.consumed", trace.WithAttributes(
		attribute.Int("inbox.count", len(consumed)),
	))
	now := time.Now().UTC()
	for _, d := range consumed {
		domain.LogBanner(a.Logger, domain.BannerRecv, string(d.Kind), d.Name, d.Description)
		if err := a.Emitter.EmitInboxConsumed(domain.InboxConsumedData{
			Name:   d.Name,
			Kind:   d.Kind,
			Source: d.Name + ".md",
		}, now); err != nil {
			return fmt.Errorf("emit inbox consumed: %w", err)
		}
	}
	return nil
}

// generateDMails runs Phase 3: creates D-Mail entities from meter candidates,
// validates them, and emits dmail-generated events.
// This produces KindImplFeedback and/or KindDesignFeedback based on divergence
// scoring (ClassifyByAxes + ResolveFeedbackKinds). Works with or without --base.
func (a *Amadeus) generateDMails(ctx context.Context, meterResult domain.MeterResult, now time.Time) ([]domain.DMail, error) {
	_, span3 := platform.Tracer.Start(ctx, "phase.dmail_generation", // nosemgrep: adr0003-otel-span-without-defer-end -- End() called per branch [permanent]
		trace.WithAttributes(
			attribute.Int("phase.number", 3),
			attribute.String("phase.name", "dmail_generation"),
		),
	)
	var dmails []domain.DMail
	quantitative := domain.ClassifyByAxes(meterResult.Divergence.Axes, a.Config.Weights)

	for _, candidate := range meterResult.DMailCandidates {
		kinds := domain.ResolveFeedbackKinds(candidate.Category, quantitative)
		for _, kind := range kinds {
			name, err := a.Store.NextDMailName(kind)
			if err != nil {
				span3.End()
				return nil, fmt.Errorf("phase 3 (dmail name): %w", err)
			}
			dmail := domain.DMail{
				SchemaVersion: domain.DMailSchemaVersion,
				Name:          name,
				Kind:          kind,
				Description:   candidate.Description,
				Issues:        candidate.Issues,
				Severity:      meterResult.Divergence.Severity,
				Action:        domain.DMailAction(candidate.Action),
				Targets:       candidate.Targets,
				Metadata: map[string]string{
					"created_at": now.Format(time.RFC3339),
				},
				Body: candidate.Detail,
			}
			if dmail.Action == "" {
				dmail.Action = domain.DefaultDMailAction(meterResult.Divergence.Severity)
			}
			if errs := domain.ValidateDMail(dmail); len(errs) > 0 {
				a.Logger.Warn("skipping invalid %s dmail %s: %v", kind, name, errs)
				continue
			}
			domain.LogBanner(a.Logger, domain.BannerSend, string(dmail.Kind), dmail.Name, dmail.Description)
			if err := a.Emitter.EmitDMailGenerated(dmail, now); err != nil {
				span3.End()
				return nil, fmt.Errorf("phase 3 (emit dmail): %w", err)
			}
			span3.AddEvent("dmail.created", trace.WithAttributes(
				attribute.String("dmail.name", dmail.Name),
				attribute.String("dmail.kind", string(dmail.Kind)),
				attribute.String("dmail.severity", string(dmail.Severity)),
			))
			dmails = append(dmails, dmail)
		}
	}
	span3.End()
	return dmails, nil
}

// detectConvergence runs Phase 4: World Line Convergence detection.
// Analyzes all D-Mails for recurring patterns, emits convergence events,
// and generates convergence D-Mails for uncovered alerts.
func (a *Amadeus) detectConvergence(now time.Time) ([]domain.ConvergenceAlert, []domain.DMail, error) {
	allDMails, convergenceErr := a.Store.LoadAllDMails()
	if convergenceErr != nil {
		return nil, nil, nil // tolerate load failure
	}
	convergenceAlerts := a.Config.DetectConvergence(allDMails, now)
	for _, alert := range convergenceAlerts {
		if err := a.Emitter.EmitConvergenceDetected(alert, now); err != nil {
			return nil, nil, fmt.Errorf("phase 4 (emit convergence event): %w", err)
		}
	}
	saved, saveErr := a.saveConvergenceDMails(convergenceAlerts)
	if saveErr != nil {
		return convergenceAlerts, nil, saveErr
	}
	return convergenceAlerts, saved, nil
}

// saveConvergenceDMails creates and saves D-Mails for convergence alerts.
// Skips alerts whose target already has an existing convergence D-Mail in the archive
// to prevent duplicate messages on repeated runs.
// Returns the saved D-Mails and any error encountered during naming or writing.
func (a *Amadeus) saveConvergenceDMails(alerts []domain.ConvergenceAlert) ([]domain.DMail, error) {
	// Load existing D-Mails for dedup (I/O)
	allDMails, err := a.Store.LoadAllDMails()
	if err != nil {
		allDMails = nil // tolerate load failure; proceed without dedup
	}

	// Filter alerts using domain logic (pure)
	uncovered := domain.FilterUncoveredConvergenceAlerts(allDMails, alerts)

	// Generate D-Mails from uncovered alerts (pure)
	convergenceDMails := domain.GenerateConvergenceDMails(uncovered)

	// Persist generated D-Mails (I/O)
	var saved []domain.DMail
	for _, cd := range convergenceDMails {
		cdName, nameErr := a.Store.NextDMailName(domain.KindConvergence)
		if nameErr != nil {
			return saved, fmt.Errorf("convergence dmail name: %w", nameErr)
		}
		cd.Name = cdName
		domain.LogBanner(a.Logger, domain.BannerSend, string(cd.Kind), cd.Name, cd.Description)
		if err := a.Emitter.EmitDMailGenerated(cd, time.Now().UTC()); err != nil {
			return saved, fmt.Errorf("emit convergence dmail %s: %w", cdName, err)
		}
		saved = append(saved, cd)
	}
	return saved, nil
}
