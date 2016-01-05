# ReadKeyServer

ReadKeyServer is the backend server behind [ReadKeyRSS](https://github.com/EDFward/ReadKeyRSS) web reader. Its primary jobs are to handle client's HTTP requests and process RSS/Atom feeds.

## Architecture

![readkey-architecture](http://i.imgur.com/Am2tQQL.png)

The server contains 4 parts (diagrams in gray), for 3 functionalities:

1. To serve the web pages and provides RESTful API for our resources (feed sources, feed items, etc). it's developed using [Gin web framework](https://github.com/gin-gonic/gin).
2. To persist data such as user subscriptions and feed source information to backend storage (for now it's Redis).
3. To retrieve feeds from RSS/Atom sites. **Feeder** manages RSS/Atom site monitoring with the help of [go-pkg-rss](https://github.com/jteeuwen/go-pkg-rss) library. When a new site needs to be monitored, feeder will spawn a new goroutine (feed handler) to keep listening and process feed items, which will later be written to the backend storage. Several custom data types are defined here, namely the feed source, feed entry and feed item, which are regarded as our resources in the web app. The feed data processing work is also done by feed handlers (like text cleaning), as well as fetching keywords (described later).

## Authentication

For now I use [Auth0](https://auth0.com/) to authenticate and authorize apps. Now it supports Google account, Github account and traditional username-password authentication.

## Keyword Extraction

For details, check [ReadKeyWord repo](https://github.com/EDFward/ReadKeyWord).

When the feed handler finds new items, it will send the content to the keyword server and store the returned keywords together with the item itself, therefore the front-end could fetch those keywords directly from the ReadKey main server rather than asking the keyword server repeatedly.
