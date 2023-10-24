package sessions

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/cufee/feedlr-yt/internal/utils"
	"github.com/gomodule/redigo/redis"
)

var defaultClient *client

type client struct {
	redis *redis.Pool
}

func newPool(uri string) *redis.Pool {
	return &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.DialURL(uri)
			if err != nil {
				return nil, err
			}
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}
}

func init() {
	defaultClient = &client{
		redis: newPool(utils.MustGetEnv("REDIS_URL")),
	}
}

func (c *client) Set(collection, key string, value interface{}) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}

	res, err := c.redis.Get().Do("SET", fmt.Sprintf("%s:%s", collection, key), encoded)
	if err != nil {
		return err
	}
	if res.(string) != "OK" {
		return errors.New("failed to set value")
	}
	return nil
}

func (c *client) Get(collection, key string, target interface{}) error {
	raw, err := redis.Bytes(c.redis.Get().Do("GET", fmt.Sprintf("%s:%s", collection, key)))
	if err != nil {
		return err
	}
	if len(raw) == 0 {
		return ErrNotFound
	}
	return json.Unmarshal(raw, target)
}

func (c *client) Del(collection, key string) error {
	res, err := c.redis.Get().Do("DEL", fmt.Sprintf("%s:%s", collection, key))
	if err != nil {
		return err
	}
	if res.(int64) == 0 {
		return ErrNotFound
	}
	return nil
}
