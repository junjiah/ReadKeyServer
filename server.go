package main

import (
	"flag"
	"strconv"

	"github.com/edfward/readkey/feeder"
	"github.com/edfward/readkey/libstore"
	"github.com/edfward/readkey/model/feed"
	"github.com/edfward/readkey/model/user"
	"github.com/edfward/readkey/util"
	"github.com/gin-gonic/gin"
)

var (
	keywordServerEndPoint = flag.String("keywordServerEndPoint", "4567/keywords", "end point of keyword server")
	redisServer           = flag.String("redisServer", ":6379", "")
	fd                    feeder.Feeder
)

// Parse command line arguments and set up libstore and ReadKey feeder.
func init() {
	flag.Parse()

	// Init feeder.
	fd = feeder.NewFeeder("http://localhost:" + *keywordServerEndPoint)
	// Init the models and backend redis store.
	rs := libstore.NewStore(*redisServer)
	user.Setup(rs)
	feed.Setup(rs)
}

func main() {

	r := gin.Default()

	// Serve static files.
	r.GET("/", func(c *gin.Context) {
		c.Redirect(302, "web/index.html")
	})
	r.Static("web", "./web")

	authorized := r.Group("/", gin.BasicAuth(gin.Accounts{
		"edfward": "123",
	}))

	// Get the list of subscribed feed sources, if successful return the list of format
	// { subscriptions: [{ id, title }] }.
	authorized.GET("subscription", func(c *gin.Context) {
		username := c.MustGet(gin.AuthUserKey).(string)
		subs := user.GetFeedSubscriptions(username)
		c.JSON(200, gin.H{"subscriptions": subs})
	})

	// Add a subscription, if successful return the subscribed feed source of format
	// { id, title }.
	authorized.POST("subscription", func(c *gin.Context) {
		c.Writer.WriteHeader(400)
		username := c.MustGet(gin.AuthUserKey).(string)
		if subUrl := c.PostForm("url"); subUrl != "" {
			src, err := fd.GetFeedSource(subUrl)
			if err != nil {
				c.JSON(400, gin.H{"error": err.Error()})
				return
			}
			feed.AddFeedSourceSubscriber(src.SourceId, username)
			user.AppendFeedSubscription(username, *src)
			// Init unread items for current user.
			user.InitUserUnreadQueue(username, src.SourceId)
			c.JSON(201, src)
		}
	})

	// Retrieve a specific subscription / feed source, if successful return the list of format
	// { feeds: [{ id, title, summary }] }.
	authorized.GET("subscription/*id", func(c *gin.Context) {
		c.Writer.WriteHeader(400)
		username := c.MustGet(gin.AuthUserKey).(string)
		// TODO: Get unread parameter from request.
		// unreadOnly := true
		if subId := c.Param("id"); subId != "/" {
			// Off-by-one to ignore the first '/'.
			subId = util.Escape(subId[1:])
			unreadIds := user.GetUnreadFeedIds(username, subId)
			entries := feed.GetFeedEntriesFromSource(subId, unreadIds)
			c.JSON(200, gin.H{"feeds": entries})
		}
	})

	// Unsubscribe a feed source.
	authorized.DELETE("subscription/*id", func(c *gin.Context) {
		c.Writer.WriteHeader(404)
		username := c.MustGet(gin.AuthUserKey).(string)
		if subId := c.Param("id"); subId != "/" {
			// Off-by-one to ignore the first '/'.
			subId = util.Escape(subId[1:])
			if success := user.RemoveFeedSubscription(username, subId); success {
				c.Writer.WriteHeader(200)
			} else {
				c.Writer.WriteHeader(404)
			}
		}
	})

	// Mark a feed item as read.
	authorized.PUT("subscription/*id", func(c *gin.Context) {
		username := c.MustGet(gin.AuthUserKey).(string)
		var form struct {
			ItemId string `form:"itemId" binding:"required"`
			Read   bool   `form:"read" binding:"required"`
		}

		if c.Bind(&form) == nil {
			srcId := util.Escape(c.Param("id")[1:])
			feedId := util.Escape(form.ItemId)
			user.RemoveUnreadFeedId(username, srcId, feedId)
			c.Writer.WriteHeader(204)
		}
	})

	// Fetch number of unread entries.
	authorized.GET("unreadcount/*id", func(c *gin.Context) {
		c.Writer.WriteHeader(400)
		username := c.MustGet(gin.AuthUserKey).(string)
		if subId := c.Param("id"); subId != "/" {
			// Off-by-one to ignore the first '/'.
			subId = util.Escape(subId[1:])
			cnt := user.GetUnreadFeedCount(username, subId)
			c.String(200, strconv.FormatInt(cnt, 10))
		}
	})

	// Retrieve the specific feed of the format { link, content } if successful.
	authorized.GET("feed/*id", func(c *gin.Context) {
		c.Writer.WriteHeader(400)
		if feedId := c.Param("id"); feedId != "/" {
			// Off-by-one to ignore the first '/'.
			feedId = util.Escape(feedId[1:])
			item := feed.GetFeed(feedId)
			c.JSON(200, item)
		}
	})

	// Listen and Server in 0.0.0.0:8080
	r.Run(":8080")
}
