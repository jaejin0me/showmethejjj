package main

import (
	"fmt"
	"log"
	"net/http"

	sessions "github.com/goincremental/negroni-sessions"
	"github.com/goincremental/negroni-sessions/cookiestore"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
	"github.com/unrolled/render"
	"github.com/urfave/negroni"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

const (
	sessionKey       = "simple_chat_session"
	sessionSecret    = "simple_chat_session_secret"
	socketBufferSize = 1024
)

var (
	renderer     *render.Render
	mongoSession *mgo.Session
	upgrader     = &websocket.Upgrader{
		ReadBufferSize:  socketBufferSize,
		WriteBufferSize: socketBufferSize,
	}
)

func init() {
	renderer = render.New()

	s, err := mgo.Dial("mongodb://localhost:27017")
	if err != nil {
		panic(err)
	}

	mongoSession = s
}

func main() {
	router := httprouter.New()

	router.GET("/", func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		// 기본페이지
		renderer.HTML(w, http.StatusOK, "index", map[string]string{"title": "Simple chat!"})
	})

	router.GET("/login", func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		// 로그인 페이지
		renderer.HTML(w, http.StatusOK, "login", nil)
	})

	router.GET("/logout", func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		// 로그아웃 페이지
		sessions.GetSession(req).Delete(currentUserKey)
		http.Redirect(w, req, "/login", http.StatusFound)
	})

	router.GET("/poems/:author/:title", func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		fmt.Println("test")
		session := mongoSession.Copy()
		defer session.Close()

		var messages []Message

		err := session.DB("kosmos").C("pomes").Find(bson.M{"title": ps.ByName("title"), "author": ps.ByName("author")}).All(&messages)

		if err != nil {
			renderer.JSON(w, http.StatusInternalServerError, err)
			return
		}

		renderer.JSON(w, http.StatusOK, messages)
	})

	router.GET("/auth/:action/:provider", loginHandler)
	router.POST("/rooms", createRoom)
	router.GET("/rooms", retrieveRooms)
	router.GET("/rooms/:id/messages", retrieveMessages)
	router.GET("/ws/:room_id", func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		socket, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Fatal("ServeHTTP:", err)
			return
		}
		newClient(socket, ps.ByName("room_id"), GetCurrentUser(r))
	})

	c := cors.New(cors.Options{
		AllowedMethods:     []string{"GET", "POST", "OPTIONS"},
		AllowedOrigins:     []string{"*"},
		AllowCredentials:   true,
		AllowedHeaders:     []string{"Content-Type", "Bearer", "Bearer ", "content-type", "Origin", "Accept"},
		OptionsPassthrough: true,
	})

	handler := c.Handler(router)

	n := negroni.Classic()
	store := cookiestore.New([]byte(sessionSecret))
	n.Use(sessions.Sessions(sessionKey, store))
	n.Use(LoginRequired("/login", "/auth", "/poems"))
	n.UseHandler(handler)

	n.Run(":3000")
}
