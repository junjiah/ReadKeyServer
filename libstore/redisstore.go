package libstore

import (
	"time"

	"github.com/garyburd/redigo/redis"
)

type RedisStrore interface {
	GetConnection() redis.Conn
}

type redisStore struct {
	pool *redis.Pool
}

func NewStore(redisServer string) *redisStore {
	// Build redis connection.
	pool := &redis.Pool{
		MaxIdle:     3,
		IdleTimeout: 240 * time.Second,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", redisServer)
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

	return &redisStore{
		pool: pool,
	}
}

func (rs *redisStore) GetConnection() redis.Conn {
	return rs.pool.Get()
}
