package main

import (
	"context"
	"github.com/geekbrains-gomicrosevices/lesson7/pkg/grpc/user"
	"github.com/geekbrains-gomicrosevices/lesson7/pkg/jwt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"gopkg.in/alexcesaro/statsd.v2"
	"log"
	"net"
	"net/http"
	"strconv"
)

var cfg = struct {
	Port     int
	HTTPPort int
}{
	Port:     8082,
	HTTPPort: 8092,
}

var Stat *statsd.Client

func main() {

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Printf("Starting http server on port %d", cfg.HTTPPort)
		http.ListenAndServe(":"+strconv.Itoa(cfg.HTTPPort), nil)
	}()

	var err error
	Stat, err = statsd.New(
		statsd.Address("graphite:8125"),
		statsd.Prefix("user"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer Stat.Close()

	srv := grpc.NewServer()

	user.RegisterUserServer(srv, &UserService{})

	listener, err := net.Listen("tcp", ":"+strconv.Itoa(cfg.Port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	log.Printf("Starting grpc server on port: %d", cfg.Port)
	srv.Serve(listener)
}

type UserService struct{}

func (s *UserService) Login(
	ctx context.Context,
	in *user.LoginRequest,
) (*user.LoginResponse, error) {

	if Stat == nil {
		log.Printf("No statsd")
	} else {
		Stat.Increment("login")
	}

	u := UU.GetByEmail(in.GetEmail())

	if u == nil {
		return &user.LoginResponse{Error: "Пользователь не найден"}, nil
	}

	if u.Pwd != in.GetPwd() {
		return &user.LoginResponse{Error: "Неправильный email или пароль"}, nil
	}

	t, err := jwt.Make(jwt.Payload{u.ID, u.Name, u.IsPaid})

	if err != nil {
		log.Printf("Token error: %v", err)
		return &user.LoginResponse{Error: "Внутренняя ошибка сервиса"}, nil
	}

	return &user.LoginResponse{Jwt: t}, nil
}

type User struct {
	ID     int
	Email  string
	Name   string
	IsPaid bool
	Pwd    string
	Token  string
}

type UserStorage []User

var UU = UserStorage{
	User{1, "bob@mail.ru", "Bob", true, "god", "1"},
	User{2, "alice@mail.ru", "Alice", false, "secret", "2"},
}

func (uu UserStorage) GetByEmail(email string) *User {
	for _, u := range uu {
		if u.Email == email {
			return &u
		}
	}
	return nil
}

type LoginRespErr struct {
	Error string `json:"error"`
}
