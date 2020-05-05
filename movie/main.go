package main

import (
	"encoding/json"
	"github.com/geekbrains-gomicrosevices/lesson7/movie/moviegrpc"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"net"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
)

var cfg = struct {
	Port     int
	HTTPPort int
}{
	Port:     8081,
	HTTPPort: 8088,
}

func main() {

	f, err := os.OpenFile("/logs/cinema_online/movie.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)

	log.SetFormatter(&log.JSONFormatter{})

	srv := grpc.NewServer(
		grpc.UnaryInterceptor(RecoverInterceptor),
	)

	moviegrpc.RegisterMovieServer(srv, &Service{})

	listener, err := net.Listen("tcp", ":"+strconv.Itoa(cfg.Port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// http.HandleFunc("/health", healthHandler)
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Printf("Starting http sever on port %d", cfg.HTTPPort)
		http.ListenAndServe(":"+strconv.Itoa(cfg.HTTPPort), nil)
	}()

	log.Printf("Starting on port %d", cfg.Port)
	log.Fatal(srv.Serve(listener))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	go func() {
		select {}
	}()

	m := &runtime.MemStats{}
	runtime.ReadMemStats(m)

	data, _ := json.Marshal(map[string]interface{}{
		"status":        "ok",
		"num_goroutine": runtime.NumGoroutine(),
		"mem":           m.HeapAlloc,
	})
	w.Write(data)
}

type Service struct{}

func (s *Service) GetMovie(
	c context.Context,
	req *moviegrpc.GetMovieRequest,
) (
	resp *moviegrpc.GetMovieResponse,
	err error,
) {
	return movie2response(MM[req.Id]), err
}

func RecoverInterceptor(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp interface{}, err error) {
	var rid string
	defer func() {
		if r := recover(); r != nil {
			log.WithFields(log.Fields{
				"request_id": rid,
			}).Printf("Recover from %v, %s", r, debug.Stack())
			err = status.Errorf(codes.Internal, "Internal error")
			return
		}
	}()

	md, _ := metadata.FromIncomingContext(ctx)
	rid = md.Get("x-request-id")[0]

	return handler(ctx, req)
}

func (s *Service) MovieList(
	ctx context.Context,
	req *moviegrpc.MovieListRequest,
) (
	resp *moviegrpc.MovieListResponse,
	err error,
) {
	panic("Oops")
	resp = new(moviegrpc.MovieListResponse)
	for _, m := range MM {
		resp.Movies = append(resp.Movies, movie2response(m))
	}
	return
}

func movie2response(m Movie) *moviegrpc.GetMovieResponse {
	return &moviegrpc.GetMovieResponse{
		Id:       int64(m.ID),
		Name:     m.Name,
		Poster:   m.Poster,
		MovieUrl: m.MovieUrl,
		IsPaid:   m.IsPaid,
	}
}

type Movie struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Poster   string `json:"poster"`
	MovieUrl string `json:"movie_url"`
	IsPaid   bool   `json:"is_paid"`
}

var MM = []Movie{
	Movie{0, "Бойцовский клуб", "/static/posters/fightclub.jpg", "https://youtu.be/qtRKdVHc-cE", true},
	Movie{1, "Крестный отец", "/static/posters/father.jpg", "https://youtu.be/ar1SHxgeZUc", false},
	Movie{2, "Криминальное чтиво", "/static/posters/pulpfiction.jpg", "https://youtu.be/s7EdQ4FqbhY", true},
}
