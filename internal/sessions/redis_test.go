package sessions

import (
	"testing"
	"time"
)

func TestRedisOops(t *testing.T) {
	data := SessionData{
		ID:        "test",
		ExpiresAt: time.Now(),
	}

	err := defaultClient.Set("test", "key1", data)
	if err != nil {
		t.Fatal(err)
	}

	var data2 []SessionData
	err = defaultClient.Get("test", "key1", &data2)
	if err != nil {
		t.Fatal(err)
	}
	if data2[0].ID != data.ID {
		t.Fatal("data2.ID != data.ID")
	}
}
