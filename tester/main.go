package main

import (
	"context"
	"github.com/geekbrains-gomicrosevices/lesson7/pkg/grpc/user"
	"google.golang.org/grpc"
	"log"
	"math/rand"
	"time"
)

func main() {
	// Инициируем рандомизатор начальным значением
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	connUser, err := grpc.Dial(":8082", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect: %s", err)
	}
	defer connUser.Close()
	userCli := user.NewUserClient(connUser)

	for {
		go func() {
			res, err := userCli.Login(
				context.Background(),
				&user.LoginRequest{Email: "bob@mail.ru", Pwd: "god"},
			)
			if err != nil {
				log.Printf("Error: %v", err)
				return
			}
			log.Printf("Response: %+v", res)
		}()
		// Спим случайное время до 1 с.
		time.Sleep(time.Duration(r.Intn(1000)) * time.Millisecond)
	}
}
