package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/cmu440-F15/paxosapp/libstore"
	rapi "github.com/cmu440-F15/readkey/api"
	"github.com/cmu440-F15/readkey/feeder"
	"github.com/cmu440-F15/readkey/util"
	"github.com/gin-gonic/gin"
)

var (
	paxosPorts            = flag.String("paxosPorts", "", "ports of all nodes")
	paxosIds              = flag.String("paxosIds", "", "IDs of all nodes")
	hostMap               = make(map[int]string)
	keywordServerEndPoint = flag.String("keywordServerEndPoint", "4567/keywords", "end point of keyword server")
)

// Parse command line arguments and set up libstore and ReadKey feeder.
func init() {
	flag.Parse()
	portList := strings.Split(*paxosPorts, ",")
	idList := strings.Split(*paxosIds, ",")

	for i, port := range portList {
		id, _ := strconv.Atoi(idList[i])
		hostMap[id] = "localhost:" + port
	}

	// Get libstore.
	ls, err := libstore.NewLibstore(hostMap)
	if err != nil {
		fmt.Println("Failed to start libstore, exit.....")
		os.Exit(1)
	}
	fd := feeder.NewFeeder(ls, "http://localhost:"+*keywordServerEndPoint)
	rapi.Setup(ls, fd)
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
		user := c.MustGet(gin.AuthUserKey).(string)
		// TODO: Ignore the error, since fetching error usually means empty subscriptions.
		subs, _ := rapi.GetSubscribedFeedSources(user)
		c.JSON(200, gin.H{"subscriptions": subs})
	})

	// Add a subscription, if successful return the subscribed feed source of format
	// { id, title }.
	authorized.POST("subscription", func(c *gin.Context) {
		c.Writer.WriteHeader(400)
		user := c.MustGet(gin.AuthUserKey).(string)
		if subUrl := c.PostForm("url"); subUrl != "" {
			if src, err := rapi.Subscribe(user, subUrl); err != nil {
				log.Println(err)
			} else {
				c.JSON(201, src)
			}
		}
	})

	// Retrieve a specific subscription / feed source, if successful return the list of format
	// { feeds: [{ id, title, summary }] }.
	authorized.GET("subscription/*id", func(c *gin.Context) {
		c.Writer.WriteHeader(400)
		user := c.MustGet(gin.AuthUserKey).(string)
		// TODO: Get unread parameter from request.
		unreadOnly := true
		if subId := c.Param("id"); subId != "/" {
			// Off-by-one to ignore the first '/'.
			subId = util.Escape(subId[1:])
			if entries, err := rapi.GetFeedItemEntries(user, subId, unreadOnly); err != nil {
				log.Println(err)
				c.JSON(404, gin.H{"error": err.Error()})
			} else {
				c.JSON(200, gin.H{"feeds": entries})
			}
		}
	})

	// Mark a feed item as read.
	authorized.PUT("subscription/*id", func(c *gin.Context) {
		user := c.MustGet(gin.AuthUserKey).(string)
		var form struct {
			ItemId string `form:"itemId" binding:"required"`
			Read   bool   `form:"read" binding:"required"`
		}

		if c.Bind(&form) == nil {
			sourceId := util.Escape(c.Param("id")[1:])
			itemId := util.Escape(form.ItemId)
			// TODO: Returned `read` is not used.
			_, err := rapi.MarkFeedItem(user, sourceId, itemId, form.Read)
			if err != nil {
				// No content.
				c.Writer.WriteHeader(400)
			} else {
				c.Writer.WriteHeader(204)
			}
		}
	})

	// Fetch number of unread entries.
	authorized.GET("unreadcount/*id", func(c *gin.Context) {
		c.Writer.WriteHeader(400)
		user := c.MustGet(gin.AuthUserKey).(string)
		if subId := c.Param("id"); subId != "/" {
			// Off-by-one to ignore the first '/'.
			subId = util.Escape(subId[1:])
			if unreadCnt, err := rapi.GetUnreadCount(user, subId); err == nil {
				c.String(200, strconv.Itoa(unreadCnt))
			}
		}
	})

	// Retrieve the specific feed of the format { link, content } if successful.
	authorized.GET("feed/*id", func(c *gin.Context) {
		c.Writer.WriteHeader(400)
		if feedId := c.Param("id"); feedId != "/" {
			// Off-by-one to ignore the first '/'.
			feedId = util.Escape(feedId[1:])
			if item, err := rapi.GetFeedItem(feedId); err != nil {
				log.Println(err)
				c.JSON(404, gin.H{"error": err.Error()})
			} else {
				c.JSON(200, item)
			}
		}
	})

	// Listen and Server in 0.0.0.0:8080
	r.Run(":8080")
}
