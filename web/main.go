package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/geekbrains-gomicrosevices/lesson7/pkg/grpc/user"
	"github.com/geekbrains-gomicrosevices/lesson7/pkg/jwt"
	"github.com/geekbrains-gomicrosevices/lesson7/pkg/render"
	"github.com/geekbrains-gomicrosevices/lesson7/web/moviegrpc"
	"github.com/gorilla/mux"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

const (
	ServicePrefix = "service/web"
	ServiceName   = "web"
)

var cfg = struct {
	Port          int
	UserAddr      string
	MovieAddr     string
	MovieGRPCPort int
	PaymentAddr   string
}{
	Port:          8080,
	MovieAddr:     "http://localhost:8081",
	MovieGRPCPort: 8081,
	UserAddr:      "http://localhost:8082",
	PaymentAddr:   "http://localhost:8083",
}

var TT struct {
	MovieList *template.Template
	Login     *template.Template
}

var UserCli user.UserClient
var MovieCli moviegrpc.MovieClient

func loadConfig(addr string) error {
	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = addr
	consul, err := consulapi.NewClient(consulConfig)
	if err != nil {
		return err
	}

	port, _, err := consul.KV().Get(ServicePrefix+"/port", nil)
	if err != nil || port == nil {
		return fmt.Errorf("Can't get '/port' value from consul for " + ServicePrefix)
	}

	cfg.Port, err = strconv.Atoi(string(port.Value))
	if err != nil {
		return fmt.Errorf("Wrong 'port' value: %w", err)
	}

	return nil
}

var RequestIDContextKey = struct{}{}

func requestIDMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := uuid.NewV4()
		rWithId := r.WithContext(context.WithValue(r.Context(), RequestIDContextKey, id.String()))
		next(w, rWithId)
	}
}

func main() {
	consulAddr := flag.String("consul_addr", "localhost:8500", "Consul address")
	flag.Parse()

	err := loadConfig(*consulAddr)
	if err != nil {
		log.Fatal(err)
	}

	f, err := os.OpenFile(
		fmt.Sprintf("/logs/cinema_online/%s.log", ServiceName),
		os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666,
	)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)

	// log.SetFormatter(&log.JSONFormatter{})

	r := mux.NewRouter()
	r.HandleFunc("/", requestIDMiddleware(MainHandler))

	r.HandleFunc("/login", LoginFormHandler).Methods("Get")
	r.HandleFunc("/login", LoginHandler).Methods("POST")
	r.HandleFunc("/logout", LogoutHandler).Methods("POST")

	connUser, err := grpc.Dial(":1234", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %s", err)
	}
	defer connUser.Close()
	UserCli = user.NewUserClient(connUser)

	connMovie, err := grpc.Dial(":"+strconv.Itoa(cfg.MovieGRPCPort), grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %s", err)
	}
	defer connMovie.Close()
	MovieCli = moviegrpc.NewMovieClient(connMovie)

	fs := http.FileServer(http.Dir("assets"))
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", fs))

	// Настройка шаблонизатора
	render.SetTemplateDir(".")
	render.SetTemplateLayout("layout.html")
	render.AddTemplate("main", "main.html")
	render.AddTemplate("login", "login.html")
	err = render.ParseTemplates()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Starting on port %d", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(cfg.Port), r))
}

type MainPage struct {
	Movies      []Movie
	MoviesError string
	User        User
	PayURL      string
}

type User struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	IsPaid bool   `json:"is_paid"`
}

type Movie struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Poster   string `json:"poster"`
	MovieUrl string `json:"movie_url"`
	IsPaid   bool   `json:"is_paid"`
}

func MainHandler(w http.ResponseWriter, r *http.Request) {
	page := MainPage{}

	ctx := r.Context()
	rid := ctx.Value(RequestIDContextKey).(string)

	var err error
	page.Movies, err = getMovies(ctx)
	if err != nil {
		log.WithFields(log.Fields{
			"request_id": rid,
		}).Printf("Get movie error: %v", err)
		page.MoviesError = "Не удалось загрузить список. Код ошибки: " + rid
	}

	page.User, err = getUserByToken(r)
	if err != nil {
		log.Printf("[%s] Get user error: %v", rid, err)
	} else {
		page.PayURL = cfg.PaymentAddr + "/checkout?uid=" + strconv.Itoa(page.User.ID)
	}

	render.RenderTemplate(w, "main", page)
}

type LoginPage struct {
	User  User
	Error string
}

func LoginFormHandler(w http.ResponseWriter, r *http.Request) {
	page := &LoginPage{}

	var err error
	page.User, err = getUserByToken(r)
	if err != nil {
		log.Printf("No user: %v", err)
		// В случае не валидного токена показываем страницу логина
		TT.Login.ExecuteTemplate(w, "base", page)
		return
	}

	TT.Login.ExecuteTemplate(w, "base", page)
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	page := &LoginPage{}

	r.ParseForm()
	email := r.PostFormValue("email")
	pwd := r.PostFormValue("pwd")

	res, err := UserCli.Login(
		context.Background(),
		&user.LoginRequest{Email: email, Pwd: pwd},
	)

	// Что-то не так с сервисом user
	if err != nil {
		log.Printf("Get user error: %v", err)
		page.Error = "Сервис авторизации не доступен"
		TT.Login.ExecuteTemplate(w, "base", page)
		return
	}

	// Ошибка логина, ее можно показать пользователю
	if res.GetError() != "" {
		page.Error = res.GetError()
		TT.Login.ExecuteTemplate(w, "base", page)
		return
	}

	tok := res.GetJwt()

	// Если пользователь успешно залогинен записываем токен в cookie
	http.SetCookie(w, &http.Cookie{Name: "jwt", Value: tok})

	jwtData, err := jwt.Parse(tok)
	if err != nil {
		// В случае не валидного токена показываем страницу логина
		TT.Login.ExecuteTemplate(w, "base", page)
		return
	}

	page.User = User{Name: jwtData.Name}
	TT.Login.ExecuteTemplate(w, "base", page)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "jwt", MaxAge: -1})
	http.Redirect(w, r, "/login", http.StatusFound)
}

func getMovies(ctx context.Context) (mm []Movie, err error) {
	rid := ctx.Value(RequestIDContextKey).(string)
	md := metadata.Pairs("X-Request-ID", rid)
	ctxRpc := metadata.NewOutgoingContext(context.Background(), md)

	res, err := MovieCli.MovieList(
		ctxRpc,
		&moviegrpc.MovieListRequest{},
	)

	if err != nil {
		log.Printf("Get movie error: %v", err)
		return nil, err
	}

	for _, m := range res.Movies {
		mm = append(mm, Movie{
			ID:       int(m.Id),
			Name:     m.Name,
			Poster:   m.Poster,
			MovieUrl: m.MovieUrl,
			IsPaid:   m.IsPaid,
		})
	}

	return
}

var ERR_NO_JWT = errors.New("No 'jwt' cookie")

func getUserByToken(r *http.Request) (u User, err error) {
	tok, err := r.Cookie("jwt")
	if tok == nil {
		return u, ERR_NO_JWT
	}

	jwtData, err := jwt.Parse(tok.Value)
	if err != nil {
		return u, fmt.Errorf("Can't parse toke: %w", err)
	}

	u.Name = jwtData.Name
	u.IsPaid = jwtData.IsPaid
	return u, err
}

func post(url string, in url.Values, out interface{}) error {
	r, err := http.DefaultClient.PostForm(url, in)
	if err != nil {
		return fmt.Errorf("make POST request error: %w", err)
	}

	return parseResponse(r, out)
}

func get(url string, out interface{}) error {
	r, err := http.DefaultClient.Get(url)
	if err != nil {
		return fmt.Errorf("make GET request error: %w", err)
	}

	return parseResponse(r, out)
}

func parseResponse(res *http.Response, out interface{}) error {
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("read response error: %w", err)
	}

	err = json.Unmarshal(body, out)
	fmt.Printf("%s", body)
	if err != nil {
		return fmt.Errorf("parse body error '%s': %w", body, err)
	}

	return nil
}
