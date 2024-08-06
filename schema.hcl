schema "main" {
}

table "auth_nonces" {
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

  column "used" {
    null = false
    type = boolean
  }
  column "expires_at" {
    null = false
    type = date
  }
  column "value" {
    null = false
    type = text
  }
  
  index "idx_auth_nonces_expires_at_used" {
    columns = [ column.id, column.expires_at, column.used ]
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

  foreign_key "views_user_id_fkey" {
    columns = [ column.user_id ]
    ref_columns = [ table.users.column.id ]
    on_delete   = CASCADE
  }
  foreign_key "views_video_id_fkey" {
    columns = [ column.video_id ]
    ref_columns = [ table.videos.column.id ]
  }

  index "idx_viewsuser_id" {
    columns = [ column.user_id]
  }
  index "idx_views_video_id_user_id" {
    columns = [  column.video_id, column.user_id]
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

table "connections" {
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

  column "type" {
    null = false
    type = text
  }
  column "user_id" {
    null = false
    type = text
  }
  foreign_key "connections_user_id_fkey" {
    columns = [ column.user_id ]
    ref_columns = [ table.users.column.id ]
    on_delete   = CASCADE
  }

  index "idx_connections_user_id" {
    columns = [ column.user_id ]
  }
  index "idx_connections_user_id_type" {
    columns = [ column.user_id, column.type ]
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