package database

import (
	"context"

	"github.com/cufee/feedlr-yt/internal/database/models"
	"github.com/lucsky/cuid"
	"github.com/volatiletech/sqlboiler/v4/boil"
)

func ensureID(value string) string {
	if value == "" {
		return cuid.New()
	}
	return value
}

func init() {
	// AuthNonce
	models.AddAuthNonceHook(boil.BeforeInsertHook, func(ctx context.Context, ce boil.ContextExecutor, an *models.AuthNonce) error {
		an.ID = ensureID(an.ID)
		return nil
	})
	models.AddAuthNonceHook(boil.BeforeUpsertHook, func(ctx context.Context, ce boil.ContextExecutor, an *models.AuthNonce) error {
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
}
