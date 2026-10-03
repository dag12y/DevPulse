package analytics

import (
	"context"
	"testing"
	"time"
)

// TestReportsNewVsReturning proves visitors first seen inside the window
// count as new while visitors seen before it count as returning.
func TestReportsNewVsReturning(t *testing.T) {
	pool := testDatabase(t)
	ctx := context.Background()
	repository := NewRepository(pool)
	now := time.Now().UTC()

	project := seedProjectWithTimezone(t, ctx, pool, "UTC")
	window := ResolveWindow(now, "UTC", 7)

	// A returning visitor: first seen in the previous window, active again now.
	returning := seedEvent(t, project.trackingID, "loyal", window.PrevStart.Add(time.Hour), "/old-home")
	if err := repository.Ingest(ctx, returning, ""); err != nil {
		t.Fatal(err)
	}
	back := seedEvent(t, project.trackingID, "loyal", window.Start.Add(time.Hour), "/back")
	back.SessionID = "sess_loyal_2"
	if err := repository.Ingest(ctx, back, ""); err != nil {
		t.Fatal(err)
	}
	// A brand-new visitor inside the current window.
	fresh := seedEvent(t, project.trackingID, "fresh", window.Start.Add(2*time.Hour), "/fresh")
	if err := repository.Ingest(ctx, fresh, ""); err != nil {
		t.Fatal(err)
	}

	summary, err := repository.Summary(ctx, project.workspaceID, project.trackingID, 7, now)
	if err != nil {
		t.Fatal(err)
	}
	if summary.UniqueVisitors != 2 {
		t.Fatalf("unique visitors = %d, want 2", summary.UniqueVisitors)
	}
	if summary.NewVisitors != 1 {
		t.Fatalf("new visitors = %d, want 1", summary.NewVisitors)
	}
	if summary.ReturningVisitors != 1 {
		t.Fatalf("returning visitors = %d, want 1", summary.ReturningVisitors)
	}
}

// TestReportsLandingPages proves sessions aggregate by entry path with
// session share, and that multi-page sessions count once.
func TestReportsLandingPages(t *testing.T) {
	pool := testDatabase(t)
	ctx := context.Background()
	repository := NewRepository(pool)
	now := time.Now().UTC()

	project := seedProjectWithTimezone(t, ctx, pool, "UTC")
	window := ResolveWindow(now, "UTC", 7)
	at := window.Start.Add(time.Hour)

	// Visitor one: two page views in one session landing on /pricing.
	first := seedEvent(t, project.trackingID, "buyer", at, "/pricing")
	if err := repository.Ingest(ctx, first, ""); err != nil {
		t.Fatal(err)
	}
	second := seedEvent(t, project.trackingID, "buyer", at.Add(time.Minute), "/pricing/checkout")
	second.SessionID = first.SessionID
	if err := repository.Ingest(ctx, second, ""); err != nil {
		t.Fatal(err)
	}
	// Visitor two: separate session landing on /blog.
	other := seedEvent(t, project.trackingID, "reader", at.Add(2*time.Minute), "/blog")
	if err := repository.Ingest(ctx, other, ""); err != nil {
		t.Fatal(err)
	}

	pages, err := repository.LandingPages(ctx, project.workspaceID, project.trackingID, 20, 7, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 2 {
		t.Fatalf("landing pages = %+v, want 2 entries", pages)
	}
	if pages[0].Path == "/pricing/checkout" {
		t.Fatalf("exit page leaked into landing pages: %+v", pages)
	}
	var pricing, blog *LandingPage
	for i := range pages {
		switch pages[i].Path {
		case "/pricing":
			pricing = &pages[i]
		case "/blog":
			blog = &pages[i]
		}
	}
	if pricing == nil || blog == nil {
		t.Fatalf("landing pages = %+v, want /pricing and /blog", pages)
	}
	if pricing.Sessions != 1 || pricing.Visitors != 1 || pricing.Share != 50 {
		t.Fatalf("pricing landing = %+v, want 1 session / 1 visitor / 50%% share", pricing)
	}
	if blog.Sessions != 1 || blog.Share != 50 {
		t.Fatalf("blog landing = %+v, want 1 session / 50%% share", blog)
	}
}

// TestReportsUTM proves source/medium/campaign group independently and
// that missing values collapse to "(not set)".
func TestReportsUTM(t *testing.T) {
	pool := testDatabase(t)
	ctx := context.Background()
	repository := NewRepository(pool)
	now := time.Now().UTC()

	project := seedProjectWithTimezone(t, ctx, pool, "UTC")
	window := ResolveWindow(now, "UTC", 7)
	at := window.Start.Add(time.Hour)

	withCampaign := func(visitor, source, medium, campaign string) Event {
		event := seedEvent(t, project.trackingID, visitor, at, "/")
		event.SessionID = "sess_" + visitor
		event.Campaign = Campaign{}
		if source != "" {
			event.Campaign.Source = &source
		}
		if medium != "" {
			event.Campaign.Medium = &medium
		}
		if campaign != "" {
			event.Campaign.Campaign = &campaign
		}
		return event
	}

	for _, event := range []Event{
		withCampaign("utm-1", "github", "social", "launch"),
		withCampaign("utm-2", "github", "social", "launch"),
		withCampaign("utm-3", "", "", ""),
	} {
		if err := repository.Ingest(ctx, event, ""); err != nil {
			t.Fatal(err)
		}
	}

	report, err := repository.UTMReport(ctx, project.workspaceID, project.trackingID, 7, now)
	if err != nil {
		t.Fatal(err)
	}
	byName := func(entries []UTMBreakdown) map[string]UTMBreakdown {
		grouped := map[string]UTMBreakdown{}
		for _, entry := range entries {
			grouped[entry.Name] = entry
		}
		return grouped
	}
	sources := byName(report.Sources)
	if sources["github"].PageViews != 2 || sources["(not set)"].PageViews != 1 {
		t.Fatalf("UTM sources = %+v, want github x2 and (not set) x1", report.Sources)
	}
	campaigns := byName(report.Campaigns)
	if campaigns["launch"].Visitors != 2 {
		t.Fatalf("UTM campaigns = %+v, want launch x2 visitors", report.Campaigns)
	}
	mediums := byName(report.Mediums)
	if mediums["social"].PageViews != 2 {
		t.Fatalf("UTM mediums = %+v, want social x2", report.Mediums)
	}
}

// TestReportsScreens proves screen and viewport pairs aggregate and that
// devices still include the pre-existing breakdowns.
func TestReportsScreens(t *testing.T) {
	pool := testDatabase(t)
	ctx := context.Background()
	repository := NewRepository(pool)
	now := time.Now().UTC()

	project := seedProjectWithTimezone(t, ctx, pool, "UTC")
	window := ResolveWindow(now, "UTC", 7)
	at := window.Start.Add(time.Hour)

	withDims := func(visitor string, screen, viewport Dimensions) Event {
		event := seedEvent(t, project.trackingID, visitor, at, "/")
		event.SessionID = "sess_" + visitor
		event.Screen = screen
		event.Viewport = viewport
		return event
	}

	for _, event := range []Event{
		withDims("desk-1", Dimensions{Width: 1920, Height: 1080}, Dimensions{Width: 1512, Height: 824}),
		withDims("desk-2", Dimensions{Width: 1920, Height: 1080}, Dimensions{Width: 1512, Height: 824}),
		withDims("mob-1", Dimensions{Width: 390, Height: 844}, Dimensions{Width: 390, Height: 700}),
	} {
		if err := repository.Ingest(ctx, event, ""); err != nil {
			t.Fatal(err)
		}
	}

	devices, err := repository.Devices(ctx, project.workspaceID, project.trackingID, 7, now)
	if err != nil {
		t.Fatal(err)
	}
	byName := func(entries []DeviceBreakdown) map[string]DeviceBreakdown {
		grouped := map[string]DeviceBreakdown{}
		for _, entry := range entries {
			grouped[entry.Name] = entry
		}
		return grouped
	}
	screens := byName(devices.Screens)
	if screens["1920×1080"].PageViews != 2 || screens["390×844"].Visitors != 1 {
		t.Fatalf("screens = %+v, want 1920×1080 x2 and 390×844 x1", devices.Screens)
	}
	viewports := byName(devices.Viewports)
	if viewports["1512×824"].PageViews != 2 {
		t.Fatalf("viewports = %+v, want 1512×824 x2", devices.Viewports)
	}
	if len(devices.Browsers) == 0 || len(devices.OperatingSystems) == 0 {
		t.Fatalf("devices lost pre-existing breakdowns: %+v", devices)
	}
}
