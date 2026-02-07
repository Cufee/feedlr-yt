schema "main" {
}


table "app_configuration" {
  schema = schema.main

  column "id" {
    null = false
    type = text
  }
  column "created_at" {
    null = false
    type = date
  }
  column "updated_at" {
    null = false
    type = date
  }
  primary_key {
    columns = [column.id]
  }

  column "version" {
    null = false
    type = integer
  }
  column "data" {
    null = false
    type = blob
  }
}

table "youtube_sync_accounts" {
  schema = schema.main

  column "id" {
    null = false
    type = text
  }
  column "created_at" {
    null = false
    type = date
  }
  column "updated_at" {
    null = false
    type = date
  }
  primary_key {
    columns = [column.id]
  }

  column "user_id" {
    null = false
    type = text
  }
  column "refresh_token_enc" {
    null = false
    type = blob
  }
  column "enc_secret_hash" {
    null = false
    type = text
  }
  column "playlist_id" {
    null = true
    type = text
  }
  column "sync_enabled" {
    null = false
    type = boolean
    default = true
  }
  column "last_feed_video_published_at" {
    null = true
    type = date
  }
  column "last_synced_at" {
    null = true
    type = date
  }
  column "last_sync_attempt_at" {
    null = true
    type = date
  }
  column "last_error" {
    null = false
    type = text
    default = ""
  }

  foreign_key "youtube_sync_accounts_user_id_fkey" {
    columns = [ column.user_id ]
    ref_columns = [ table.users.column.id ]
    on_delete   = CASCADE
  }

  index "idx_youtube_sync_accounts_user_id_unique" {
    columns = [ column.user_id ]
    unique = true
  }
  index "idx_youtube_sync_accounts_sync_enabled_last_synced_at" {
    columns = [ column.sync_enabled, column.last_synced_at ]
  }
}

table "youtube_tv_sync_accounts" {
  schema = schema.main

  column "id" {
    null = false
    type = text
  }
  column "created_at" {
    null = false
    type = date
  }
  column "updated_at" {
    null = false
    type = date
  }
  primary_key {
    columns = [column.id]
  }

  column "user_id" {
    null = false
    type = text
  }
  column "screen_id" {
    null = false
    type = text
  }
  column "screen_name" {
    null = false
    type = text
    default = ""
  }
  column "lounge_token_enc" {
    null = false
    type = blob
  }
  column "enc_secret_hash" {
    null = false
    type = text
  }
  column "sync_enabled" {
    null = false
    type = boolean
    default = true
  }
  column "connection_state" {
    null = false
    type = text
    default = "disconnected"
  }
  column "state_reason" {
    null = false
    type = text
    default = ""
  }
  column "last_connected_at" {
    null = true
    type = date
  }
  column "last_event_at" {
    null = true
    type = date
  }
  column "last_disconnect_at" {
    null = true
    type = date
  }
  column "last_user_activity_at" {
    null = true
    type = date
  }
  column "last_video_id" {
    null = true
    type = text
  }
  column "last_error" {
    null = false
    type = text
    default = ""
  }

  foreign_key "youtube_tv_sync_accounts_user_id_fkey" {
    columns = [ column.user_id ]
    ref_columns = [ table.users.column.id ]
    on_delete   = CASCADE
  }

  index "idx_youtube_tv_sync_accounts_user_id_unique" {
    columns = [ column.user_id ]
    unique = true
  }
  index "idx_youtube_tv_sync_accounts_sync_enabled_connection_state" {
    columns = [ column.sync_enabled, column.connection_state ]
  }
  index "idx_youtube_tv_sync_accounts_last_event_at" {
    columns = [ column.last_event_at ]
  }
}

table "channels" {
  schema = schema.main

  column "id" {
    null = false
    type = text
  }
  column "created_at" {
    null = false
    type = date
  }
  column "updated_at" {
    null = false
    type = date
  }
  primary_key {
    columns = [column.id]
  }

  column "title" {
    null = false
    type = text
  }
  column "description" {
    null = false
    type = text
  }
  column "thumbnail" {
    null = false
    type = text
    default = ""
  }

  column "feed_updated_at" {
    null = false
    type = date
    default = 0
  }
  column "uploads_playlist_id" {
    null = false
    type = text
    default = ""
  }
}

table "videos" {
  schema = schema.main

  column "id" {
    null = false
    type = text
  }
  column "created_at" {
    null = false
    type = date
  }
  column "updated_at" {
    null = false
    type = date
  }
  primary_key {
    columns = [column.id]
  }

  column "title" {
    null = false
    type = text
  }
  column "description" {
    null = false
    type = text
  }
  column "duration" {
    null = false
    type = integer
  }
  column "published_at" {
    null = false
    type = date
  }
  column "private" {
    null = false
    type = boolean
  }
  column "type" {
    null = false
    type = text
  }

  column "channel_id" {
    null = false
    type = text
  }
  foreign_key "videos_channel_id_fkey" {
    columns = [ column.channel_id ]
    ref_columns = [ table.channels.column.id ]
    on_delete   = CASCADE
  }

  index "idx_videos_published_at" {
    columns = [ column.published_at]
  }
  index "idx_videos_channel_id" {
    columns = [ column.channel_id]
  }
  index "idx_videos_published_at_channel_id" {
    columns = [ column.published_at, column.channel_id ]
  }
}

table "views" {
  schema = schema.main

  column "id" {
    null = false
    type = text
  }
  column "created_at" {
    null = false
    type = date
  }
  column "updated_at" {
    null = false
    type = date
  }
  primary_key {
    columns = [column.id]
  }

  column "user_id" {
    null = false
    type = text
  }
  column "video_id" {
    null = false
    type = text
  }
  column "progress" {
    null = false
    type = integer
  }
  column "hidden" {
    null = true
    type = boolean
    default = false
  }

  foreign_key "views_user_id_fkey" {
    columns = [ column.user_id ]
    ref_columns = [ table.users.column.id ]
    on_delete   = CASCADE
  }
  foreign_key "views_video_id_fkey" {
    columns = [ column.video_id ]
    ref_columns = [ table.videos.column.id ]
  }

  index "idx_views_user_id" {
    columns = [ column.user_id]
  }
  index "idx_views_user_id_hidden" {
    columns = [ column.user_id, column.hidden]
  }
  index "idx_views_video_id_user_id" {
    columns = [  column.video_id, column.user_id ]
    unique = true
  }
  index "idx_views_video_id_user_id_hidden" {
    columns = [  column.video_id, column.user_id, column.hidden ]
  }
}

table "users" {
  schema = schema.main

  column "id" {
    null = false
    type = text
  }
  column "created_at" {
    null = false
    type = date
  }
  column "updated_at" {
    null = false
    type = date
  }
  primary_key {
    columns = [column.id]
  }

  column "permissions" {
    null = false
    type = text
    default = ""
  }
  column "username" {
    null = false
    type = text
  }
  index "idx_users_username" {
    columns = [ column.username ]
    unique = true
  }
}

table "passkeys" {
  schema = schema.main

  column "id" {
    null = false
    type = text
  }
  column "created_at" {
    null = false
    type = date
  }
  column "updated_at" {
    null = false
    type = date
  }
  primary_key {
    columns = [column.id]
  }

  column "label" {
    null = false
    type = text
    default = ""
  }
  column "data" {
    null = false
    type = blob
  }
  column "user_id" {
    null = false
    type = text
  }

  index "idx_passkeys_user_id" {
    columns = [ column.user_id ]
  }
}

table "sessions" {
  schema = schema.main

  column "id" {
    null = false
    type = text
  }
  column "created_at" {
    null = false
    type = date
  }
  column "updated_at" {
    null = false
    type = date
  }
  primary_key {
    columns = [column.id]
  }

  column "user_id" {
    null = true
    type = text
  }
  column "connection_id" {
    null = true
    type = text
  }
  column "expires_at" {
    null = false
    type = date
  }
  column "last_used" {
    null = false
    type = date
  }
  column "deleted" {
    null = false
    type = boolean
    default = false
  }

  column "meta" {
    null = false
    type = blob
    default = ""
  }

  index "idx_sessions_id_expires_at_deleted" {
    columns = [  column.id, column.expires_at, column.deleted]
  }
  index "idx_sessions_user_id_expires_at_deleted" {
    columns = [  column.user_id, column.expires_at, column.deleted]
  }
  index "idx_sessions_user_id_last_used_deleted" {
    columns = [  column.user_id, column.last_used, column.deleted ]
  }
}

table "settings" {
  schema = schema.main

  column "id" {
    null = false
    type = text
  }
  column "created_at" {
    null = false
    type = date
  }
  column "updated_at" {
    null = false
    type = date
  }
  primary_key {
    columns = [column.id]
  }

  column "data" {
    null = false
    type = blob
  }

  column "user_id" {
    null = false
    type = text
  }
  foreign_key "settings_user_id_fkey" {
    columns = [ column.user_id ]
    ref_columns = [ table.users.column.id ]
    on_delete   = CASCADE
  }

  index "idx_settings_user_id" {
    columns = [ column.user_id ]
  }
  index "idx_settings_id_user_id_unique" {
    columns = [ column.id, column.user_id ]
    unique = true
  }
}

table "subscriptions" {
  schema = schema.main

  column "id" {
    null = false
    type = text
  }
  column "created_at" {
    null = false
    type = date
  }
  column "updated_at" {
    null = false
    type = date
  }
  primary_key {
    columns = [column.id]
  }

  column "favorite" {
    null = false
    type = boolean
  }

  column "channel_id" {
    null = false
    type = text
  }
  column "user_id" {
    null = false
    type = text
  }
  foreign_key "subscriptions_channel_id_fkey" {
    columns = [ column.channel_id ]
    ref_columns = [ table.channels.column.id ]
    on_delete   = CASCADE
  }
  foreign_key "subscriptions_user_id_fkey" {
    columns = [ column.user_id ]
    ref_columns = [ table.users.column.id ]
    on_delete   = CASCADE
  }

  index "idx_subscriptions_user_id" {
    columns = [ column.user_id ]
  }
  index "idx_subscriptions_user_id_favorite" {
    columns = [ column.user_id, column.favorite ]
  }
  index "idx_subscriptions_user_id_channel_id_unique" {
    columns = [ column.user_id, column.channel_id ]
    unique = true
  }
  index "idx_subscriptions_channel_id" {
    columns = [ column.channel_id ]
  }
}

table "playlists" {
  schema = schema.main

  column "id" {
    null = false
    type = text
  }
  column "created_at" {
    null = false
    type = date
  }
  column "updated_at" {
    null = false
    type = date
  }
  primary_key {
    columns = [column.id]
  }

  column "user_id" {
    null = false
    type = text
  }
  column "slug" {
    null = false
    type = text
  }
  column "name" {
    null = false
    type = text
  }
  column "system" {
    null = false
    type = boolean
    default = false
  }
  column "ttl_days" {
    null = true
    type = integer
  }
  column "max_size" {
    null = true
    type = integer
  }

  foreign_key "playlists_user_id_fkey" {
    columns = [ column.user_id ]
    ref_columns = [ table.users.column.id ]
    on_delete   = CASCADE
  }

  index "idx_playlists_user_id" {
    columns = [ column.user_id ]
  }
  index "idx_playlists_user_id_slug_unique" {
    columns = [ column.user_id, column.slug ]
    unique = true
  }
}

table "playlist_items" {
  schema = schema.main

  column "id" {
    null = false
    type = text
  }
  column "created_at" {
    null = false
    type = date
  }
  column "updated_at" {
    null = false
    type = date
  }
  primary_key {
    columns = [column.id]
  }

  column "playlist_id" {
    null = false
    type = text
  }
  column "video_id" {
    null = false
    type = text
  }
  column "position" {
    null = false
    type = integer
    default = 0
  }

  foreign_key "playlist_items_playlist_id_fkey" {
    columns = [ column.playlist_id ]
    ref_columns = [ table.playlists.column.id ]
    on_delete   = CASCADE
  }
  foreign_key "playlist_items_video_id_fkey" {
    columns = [ column.video_id ]
    ref_columns = [ table.videos.column.id ]
  }

  index "idx_playlist_items_playlist_id" {
    columns = [ column.playlist_id ]
  }
  index "idx_playlist_items_playlist_id_video_id_unique" {
    columns = [ column.playlist_id, column.video_id ]
    unique = true
  }
  index "idx_playlist_items_playlist_id_created_at" {
    columns = [ column.playlist_id, column.created_at ]
  }
}
