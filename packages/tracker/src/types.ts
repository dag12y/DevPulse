export const TRACKER_VERSION = "0.2.0";

export interface TrackerConfig {
  projectId: string;
  endpoint: string;
  respectDNT?: boolean;
}

export interface Dimensions {
  width: number;
  height: number;
}

export interface PageViewEvent {
  event_id: string;
  type: "page_view";
  project_id: string;
  visitor_id: string;
  session_id: string;
  timestamp: string;
  sdk_version?: string;
  page: { url: string; path: string; title: string; referrer: string };
  screen: Dimensions;
  viewport: Dimensions;
  language: string;
  timezone: string;
  campaign: {
    source?: string;
    medium?: string;
    campaign?: string;
    term?: string;
    content?: string;
  };
}

export interface Tracker {
  trackPageView(): void;
}
