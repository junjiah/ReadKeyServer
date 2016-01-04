package main

import (
	_ "crypto/sha512"
	"encoding/base64"
	"flag"
	"os"
	"strconv"

	"github.com/edfward/readkey/feeder"
	"github.com/edfward/readkey/libstore"
	"github.com/edfward/readkey/model/feed"
	"github.com/edfward/readkey/model/user"
	"github.com/edfward/readkey/util"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
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

// Middleware for authentication using Auth0.
func tokenAuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if sessions.Default(c).Get("userid") == nil {
			c.Redirect(302, "/login")
		} else {
			c.Next()
		}
	}
}

func main() {
	r := gin.Default()
	store := sessions.NewCookieStore([]byte("edfward-secret"))
	r.Use(sessions.Sessions("readkey-session", store))

	// Login endpoint.
	r.StaticFile("login", "./web/login.html")

	// Auto0 callbacks. From https://auth0.com/docs/server-platforms/golang#go-web-app-tutorial
	r.GET("callback", func(c *gin.Context) {
		domain := os.Getenv("AUTH0_DOMAIN")
		clientSecret := os.Getenv("AUTH0_CLIENT_SECRET")

		// Instantiating the OAuth2 package to exchange the Code for a Token.
		conf := &oauth2.Config{
			ClientID:     os.Getenv("AUTH0_CLIENT_ID"),
			ClientSecret: clientSecret,
			RedirectURL:  os.Getenv("AUTH0_CALLBACK_URL"),
			Scopes:       []string{"openid"},
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://" + domain + "/authorize",
				TokenURL: "https://" + domain + "/oauth/token",
			},
		}

		// Getting the Code that we got from Auth0.
		code := c.Query("code")

		// Exchanging the code for a token.
		token, err := conf.Exchange(oauth2.NoContext, code)
		if err != nil {
			c.String(500, err.Error())
			return
		}

		// Write to session.
		session := sessions.Default(c)
		idToken := token.Extra("id_token").(string)
		// From Auth0's Documentation -> Backend/API -> Go.
		parsedToken, err := jwt.Parse(idToken, func(token *jwt.Token) (interface{}, error) {
			decoded, _ := base64.URLEncoding.DecodeString(clientSecret)
			return decoded, nil
		})
		if err == nil && parsedToken.Valid {
			// Set the value in session, which looks like 'github|123456'.
			session.Set("userid", parsedToken.Claims["sub"].(string))
		} else {
			c.String(500, "parsing id token failed")
			return
		}
		if err := session.Save(); err != nil {
			c.String(500, err.Error())
			return
		}
		c.Redirect(302, "/")
	})

	authorized := r.Group("/")
	// Use auth middleware.
	authorized.Use(tokenAuthRequired())
	{
		// Serve static files.
		authorized.StaticFile("/", "./web/index.html")
		authorized.StaticFile("/app.js", "./web/app.js")
		authorized.StaticFile("/style.css", "./web/style.css")
		authorized.Static("/assets", "./web/assets")

		// Logout endporint.
		authorized.GET("logout", func(c *gin.Context) {
			session := sessions.Default(c)
			session.Clear()
			session.Save()
			c.String(200, "You Have Successfully Logged Out.")
		})

		// Get the list of subscribed feed sources, if successful return the list of format
		// { subscriptions: [{ id, title }] }.
		authorized.GET("subscription", func(c *gin.Context) {
			username := sessions.Default(c).Get("userid").(string)
			subs := user.GetFeedSubscriptions(username)
			c.JSON(200, gin.H{"subscriptions": subs})
		})

		// Add a subscription, if successful return the subscribed feed source of format
		// { id, title }.
		authorized.POST("subscription", func(c *gin.Context) {
			c.Writer.WriteHeader(400)
			username := sessions.Default(c).Get("userid").(string)
			if subURL := c.PostForm("url"); subURL != "" {
				src, err := fd.GetFeedSource(subURL)
				if err != nil {
					c.JSON(400, gin.H{"error": err.Error()})
					return
				}
				if ok := user.AppendFeedSubscription(username, src); ok {
					feed.AddFeedSourceSubscriber(src.SourceId, username)
					// Init unread items for current user.
					user.InitUserUnreadQueue(username, src.SourceId)
					c.JSON(201, src)
				} else {
					// Duplicates or error.
					c.JSON(409, gin.H{"error": "duplicate subscription or storage error"})
				}
			}
		})

		// Retrieve a specific subscription / feed source, if successful return the list of format
		// { feeds: [{ id, title, summary }] }.
		authorized.GET("subscription/*id", func(c *gin.Context) {
			c.Writer.WriteHeader(400)
			username := sessions.Default(c).Get("userid").(string)
			// TODO: Get unread parameter from request.
			// unreadOnly := true
			if subID := c.Param("id"); subID != "/" {
				// Off-by-one to ignore the first '/'.
				subID = util.Escape(subID[1:])
				unreadIds := user.GetUnreadFeedIds(username, subID)
				entries := feed.GetFeedEntriesFromSource(subID, unreadIds)
				c.JSON(200, gin.H{"feeds": entries})
			}
		})

		// Unsubscribe a feed source.
		authorized.DELETE("subscription/*id", func(c *gin.Context) {
			c.Writer.WriteHeader(404)
			username := sessions.Default(c).Get("userid").(string)
			if subID := c.Param("id"); subID != "/" {
				// Off-by-one to ignore the first '/'.
				subID = util.Escape(subID[1:])
				if success := user.RemoveFeedSubscription(username, subID); success {
					c.Writer.WriteHeader(200)
				} else {
					c.Writer.WriteHeader(404)
				}
			}
		})

		// Mark a feed item as read.
		authorized.PUT("subscription/*id", func(c *gin.Context) {
			username := sessions.Default(c).Get("userid").(string)
			var form struct {
				ItemID string `form:"itemId" binding:"required"`
				Read   bool   `form:"read" binding:"required"`
			}

			if c.Bind(&form) == nil {
				srcID := util.Escape(c.Param("id")[1:])
				feedID := util.Escape(form.ItemID)
				user.RemoveUnreadFeedId(username, srcID, feedID)
				c.Writer.WriteHeader(204)
			}
		})

		// Fetch number of unread entries.
		authorized.GET("unreadcount/*id", func(c *gin.Context) {
			c.Writer.WriteHeader(400)
			username := sessions.Default(c).Get("userid").(string)
			if subID := c.Param("id"); subID != "/" {
				// Off-by-one to ignore the first '/'.
				subID = util.Escape(subID[1:])
				cnt := user.GetUnreadFeedCount(username, subID)
				c.String(200, strconv.FormatInt(cnt, 10))
			}
		})

		// Retrieve the specific feed of the format { link, content } if successful.
		authorized.GET("feed/*id", func(c *gin.Context) {
			c.Writer.WriteHeader(400)
			if feedID := c.Param("id"); feedID != "/" {
				// Off-by-one to ignore the first '/'.
				feedID = util.Escape(feedID[1:])
				item := feed.GetFeed(feedID)
				c.JSON(200, item)
			}
		})
	}

	// Listen and Server in 0.0.0.0:8080
	r.Run(":8080")
}
