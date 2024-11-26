// Code generated by SQLBoiler 4.16.2 (https://github.com/volatiletech/sqlboiler). DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import "testing"

// This test suite runs each operation test in parallel.
// Example, if your database has 3 tables, the suite will run:
// table1, table2 and table3 Delete in parallel
// table1, table2 and table3 Insert in parallel, and so forth.
// It does NOT run each operation group in parallel.
// Separating the tests thusly grants avoidance of Postgres deadlocks.
func TestParent(t *testing.T) {
	t.Run("AppConfigurations", testAppConfigurations)
	t.Run("Channels", testChannels)
	t.Run("Passkeys", testPasskeys)
	t.Run("Sessions", testSessions)
	t.Run("Settings", testSettings)
	t.Run("Subscriptions", testSubscriptions)
	t.Run("Users", testUsers)
	t.Run("Videos", testVideos)
	t.Run("Views", testViews)
}

func TestDelete(t *testing.T) {
	t.Run("AppConfigurations", testAppConfigurationsDelete)
	t.Run("Channels", testChannelsDelete)
	t.Run("Passkeys", testPasskeysDelete)
	t.Run("Sessions", testSessionsDelete)
	t.Run("Settings", testSettingsDelete)
	t.Run("Subscriptions", testSubscriptionsDelete)
	t.Run("Users", testUsersDelete)
	t.Run("Videos", testVideosDelete)
	t.Run("Views", testViewsDelete)
}

func TestQueryDeleteAll(t *testing.T) {
	t.Run("AppConfigurations", testAppConfigurationsQueryDeleteAll)
	t.Run("Channels", testChannelsQueryDeleteAll)
	t.Run("Passkeys", testPasskeysQueryDeleteAll)
	t.Run("Sessions", testSessionsQueryDeleteAll)
	t.Run("Settings", testSettingsQueryDeleteAll)
	t.Run("Subscriptions", testSubscriptionsQueryDeleteAll)
	t.Run("Users", testUsersQueryDeleteAll)
	t.Run("Videos", testVideosQueryDeleteAll)
	t.Run("Views", testViewsQueryDeleteAll)
}

func TestSliceDeleteAll(t *testing.T) {
	t.Run("AppConfigurations", testAppConfigurationsSliceDeleteAll)
	t.Run("Channels", testChannelsSliceDeleteAll)
	t.Run("Passkeys", testPasskeysSliceDeleteAll)
	t.Run("Sessions", testSessionsSliceDeleteAll)
	t.Run("Settings", testSettingsSliceDeleteAll)
	t.Run("Subscriptions", testSubscriptionsSliceDeleteAll)
	t.Run("Users", testUsersSliceDeleteAll)
	t.Run("Videos", testVideosSliceDeleteAll)
	t.Run("Views", testViewsSliceDeleteAll)
}

func TestExists(t *testing.T) {
	t.Run("AppConfigurations", testAppConfigurationsExists)
	t.Run("Channels", testChannelsExists)
	t.Run("Passkeys", testPasskeysExists)
	t.Run("Sessions", testSessionsExists)
	t.Run("Settings", testSettingsExists)
	t.Run("Subscriptions", testSubscriptionsExists)
	t.Run("Users", testUsersExists)
	t.Run("Videos", testVideosExists)
	t.Run("Views", testViewsExists)
}

func TestFind(t *testing.T) {
	t.Run("AppConfigurations", testAppConfigurationsFind)
	t.Run("Channels", testChannelsFind)
	t.Run("Passkeys", testPasskeysFind)
	t.Run("Sessions", testSessionsFind)
	t.Run("Settings", testSettingsFind)
	t.Run("Subscriptions", testSubscriptionsFind)
	t.Run("Users", testUsersFind)
	t.Run("Videos", testVideosFind)
	t.Run("Views", testViewsFind)
}

func TestBind(t *testing.T) {
	t.Run("AppConfigurations", testAppConfigurationsBind)
	t.Run("Channels", testChannelsBind)
	t.Run("Passkeys", testPasskeysBind)
	t.Run("Sessions", testSessionsBind)
	t.Run("Settings", testSettingsBind)
	t.Run("Subscriptions", testSubscriptionsBind)
	t.Run("Users", testUsersBind)
	t.Run("Videos", testVideosBind)
	t.Run("Views", testViewsBind)
}

func TestOne(t *testing.T) {
	t.Run("AppConfigurations", testAppConfigurationsOne)
	t.Run("Channels", testChannelsOne)
	t.Run("Passkeys", testPasskeysOne)
	t.Run("Sessions", testSessionsOne)
	t.Run("Settings", testSettingsOne)
	t.Run("Subscriptions", testSubscriptionsOne)
	t.Run("Users", testUsersOne)
	t.Run("Videos", testVideosOne)
	t.Run("Views", testViewsOne)
}

func TestAll(t *testing.T) {
	t.Run("AppConfigurations", testAppConfigurationsAll)
	t.Run("Channels", testChannelsAll)
	t.Run("Passkeys", testPasskeysAll)
	t.Run("Sessions", testSessionsAll)
	t.Run("Settings", testSettingsAll)
	t.Run("Subscriptions", testSubscriptionsAll)
	t.Run("Users", testUsersAll)
	t.Run("Videos", testVideosAll)
	t.Run("Views", testViewsAll)
}

func TestCount(t *testing.T) {
	t.Run("AppConfigurations", testAppConfigurationsCount)
	t.Run("Channels", testChannelsCount)
	t.Run("Passkeys", testPasskeysCount)
	t.Run("Sessions", testSessionsCount)
	t.Run("Settings", testSettingsCount)
	t.Run("Subscriptions", testSubscriptionsCount)
	t.Run("Users", testUsersCount)
	t.Run("Videos", testVideosCount)
	t.Run("Views", testViewsCount)
}

func TestHooks(t *testing.T) {
	t.Run("AppConfigurations", testAppConfigurationsHooks)
	t.Run("Channels", testChannelsHooks)
	t.Run("Passkeys", testPasskeysHooks)
	t.Run("Sessions", testSessionsHooks)
	t.Run("Settings", testSettingsHooks)
	t.Run("Subscriptions", testSubscriptionsHooks)
	t.Run("Users", testUsersHooks)
	t.Run("Videos", testVideosHooks)
	t.Run("Views", testViewsHooks)
}

func TestInsert(t *testing.T) {
	t.Run("AppConfigurations", testAppConfigurationsInsert)
	t.Run("AppConfigurations", testAppConfigurationsInsertWhitelist)
	t.Run("Channels", testChannelsInsert)
	t.Run("Channels", testChannelsInsertWhitelist)
	t.Run("Passkeys", testPasskeysInsert)
	t.Run("Passkeys", testPasskeysInsertWhitelist)
	t.Run("Sessions", testSessionsInsert)
	t.Run("Sessions", testSessionsInsertWhitelist)
	t.Run("Settings", testSettingsInsert)
	t.Run("Settings", testSettingsInsertWhitelist)
	t.Run("Subscriptions", testSubscriptionsInsert)
	t.Run("Subscriptions", testSubscriptionsInsertWhitelist)
	t.Run("Users", testUsersInsert)
	t.Run("Users", testUsersInsertWhitelist)
	t.Run("Videos", testVideosInsert)
	t.Run("Videos", testVideosInsertWhitelist)
	t.Run("Views", testViewsInsert)
	t.Run("Views", testViewsInsertWhitelist)
}

func TestReload(t *testing.T) {
	t.Run("AppConfigurations", testAppConfigurationsReload)
	t.Run("Channels", testChannelsReload)
	t.Run("Passkeys", testPasskeysReload)
	t.Run("Sessions", testSessionsReload)
	t.Run("Settings", testSettingsReload)
	t.Run("Subscriptions", testSubscriptionsReload)
	t.Run("Users", testUsersReload)
	t.Run("Videos", testVideosReload)
	t.Run("Views", testViewsReload)
}

func TestReloadAll(t *testing.T) {
	t.Run("AppConfigurations", testAppConfigurationsReloadAll)
	t.Run("Channels", testChannelsReloadAll)
	t.Run("Passkeys", testPasskeysReloadAll)
	t.Run("Sessions", testSessionsReloadAll)
	t.Run("Settings", testSettingsReloadAll)
	t.Run("Subscriptions", testSubscriptionsReloadAll)
	t.Run("Users", testUsersReloadAll)
	t.Run("Videos", testVideosReloadAll)
	t.Run("Views", testViewsReloadAll)
}

func TestSelect(t *testing.T) {
	t.Run("AppConfigurations", testAppConfigurationsSelect)
	t.Run("Channels", testChannelsSelect)
	t.Run("Passkeys", testPasskeysSelect)
	t.Run("Sessions", testSessionsSelect)
	t.Run("Settings", testSettingsSelect)
	t.Run("Subscriptions", testSubscriptionsSelect)
	t.Run("Users", testUsersSelect)
	t.Run("Videos", testVideosSelect)
	t.Run("Views", testViewsSelect)
}

func TestUpdate(t *testing.T) {
	t.Run("AppConfigurations", testAppConfigurationsUpdate)
	t.Run("Channels", testChannelsUpdate)
	t.Run("Passkeys", testPasskeysUpdate)
	t.Run("Sessions", testSessionsUpdate)
	t.Run("Settings", testSettingsUpdate)
	t.Run("Subscriptions", testSubscriptionsUpdate)
	t.Run("Users", testUsersUpdate)
	t.Run("Videos", testVideosUpdate)
	t.Run("Views", testViewsUpdate)
}

func TestSliceUpdateAll(t *testing.T) {
	t.Run("AppConfigurations", testAppConfigurationsSliceUpdateAll)
	t.Run("Channels", testChannelsSliceUpdateAll)
	t.Run("Passkeys", testPasskeysSliceUpdateAll)
	t.Run("Sessions", testSessionsSliceUpdateAll)
	t.Run("Settings", testSettingsSliceUpdateAll)
	t.Run("Subscriptions", testSubscriptionsSliceUpdateAll)
	t.Run("Users", testUsersSliceUpdateAll)
	t.Run("Videos", testVideosSliceUpdateAll)
	t.Run("Views", testViewsSliceUpdateAll)
}
