package database

import (
	"context"

	"github.com/aarondl/sqlboiler/v4/boil"
	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/lucsky/cuid"
)

func ensureID(value string) string {
	if value == "" {
		return cuid.New()
	}
	return value
}

func init() {
	// Passkeys
	models.AddPasskeyHook(boil.BeforeInsertHook, func(ctx context.Context, ce boil.ContextExecutor, an *models.Passkey) error {
		an.ID = ensureID(an.ID)
		return nil
	})
	models.AddPasskeyHook(boil.BeforeUpsertHook, func(ctx context.Context, ce boil.ContextExecutor, an *models.Passkey) error {
		an.ID = ensureID(an.ID)
		return nil
	})
	// Sessions
	models.AddSessionHook(boil.BeforeInsertHook, func(ctx context.Context, ce boil.ContextExecutor, an *models.Session) error {
		an.ID = ensureID(an.ID)
		return nil
	})
	models.AddSessionHook(boil.BeforeUpsertHook, func(ctx context.Context, ce boil.ContextExecutor, an *models.Session) error {
		an.ID = ensureID(an.ID)
		return nil
	})
	// Channels
	models.AddChannelHook(boil.BeforeInsertHook, func(ctx context.Context, ce boil.ContextExecutor, c *models.Channel) error {
		c.ID = ensureID(c.ID)
		return nil
	})
	models.AddChannelHook(boil.BeforeUpsertHook, func(ctx context.Context, ce boil.ContextExecutor, c *models.Channel) error {
		c.ID = ensureID(c.ID)
		return nil
	})
	// Settings
	models.AddSettingHook(boil.BeforeInsertHook, func(ctx context.Context, ce boil.ContextExecutor, c *models.Setting) error {
		c.ID = ensureID(c.ID)
		return nil
	})
	models.AddSettingHook(boil.BeforeUpsertHook, func(ctx context.Context, ce boil.ContextExecutor, c *models.Setting) error {
		c.ID = ensureID(c.ID)
		return nil
	})
	// Subscriptions
	models.AddSubscriptionHook(boil.BeforeInsertHook, func(ctx context.Context, ce boil.ContextExecutor, c *models.Subscription) error {
		c.ID = ensureID(c.ID)
		return nil
	})
	models.AddSubscriptionHook(boil.BeforeUpsertHook, func(ctx context.Context, ce boil.ContextExecutor, c *models.Subscription) error {
		c.ID = ensureID(c.ID)
		return nil
	})
	// Users
	models.AddUserHook(boil.BeforeInsertHook, func(ctx context.Context, ce boil.ContextExecutor, c *models.User) error {
		c.ID = ensureID(c.ID)
		return nil
	})
	models.AddUserHook(boil.BeforeUpsertHook, func(ctx context.Context, ce boil.ContextExecutor, c *models.User) error {
		c.ID = ensureID(c.ID)
		return nil
	})
	// Videos
	models.AddVideoHook(boil.BeforeInsertHook, func(ctx context.Context, ce boil.ContextExecutor, c *models.Video) error {
		c.ID = ensureID(c.ID)
		return nil
	})
	models.AddVideoHook(boil.BeforeUpsertHook, func(ctx context.Context, ce boil.ContextExecutor, c *models.Video) error {
		c.ID = ensureID(c.ID)
		return nil
	})
	models.AddViewHook(boil.BeforeInsertHook, func(ctx context.Context, ce boil.ContextExecutor, c *models.View) error {
		c.ID = ensureID(c.ID)
		return nil
	})
	models.AddViewHook(boil.BeforeUpsertHook, func(ctx context.Context, ce boil.ContextExecutor, c *models.View) error {
		c.ID = ensureID(c.ID)
		return nil
	})

}
