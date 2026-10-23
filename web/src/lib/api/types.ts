import type { components } from "./schema";

type Schemas = components["schemas"];
export type MediaSource = "all" | "youtube" | "podcasts";
export type Channel = Schemas["Channel"];
export type MediaItem = Schemas["MediaItem"];
export type Playlist = Schemas["Playlist"];
export type Page = Schemas["Page"];
export interface PageResult<T = MediaItem> {
  items: T[];
  page: Page;
}
export type Feed = Schemas["Feed"];
export type Session = Schemas["Session"];
export type PodcastSegment = Schemas["PodcastSegment"];
export type PodcastAnalysis = Schemas["PodcastAnalysis"];
export type MediaDetail = Schemas["MediaDetail"];
export type ChannelDetail = Schemas["ChannelDetail"];
export type PlaylistDetail = Schemas["PlaylistDetail"];
export type Category = Schemas["Category"];
export type SkipSettings = Schemas["SegmentSettings"];
export type YouTubeSync = Schemas["YouTubeStatus"];
export type TVSync = Schemas["TVStatus"];
export type Settings = Schemas["Settings"];
export type MediaState = Schemas["MediaStatePatch"];
