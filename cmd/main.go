package main

import (
	"google.golang.org/grpc"
	"grpc_image_test/internal/pkg/pb"
	"grpc_image_test/internal/pkg/server"
	"log"
	"net"
	"sync"
)

func main() {
	s := grpc.NewServer()
	imageStore := server.NewDiskImageStorage(server.ImageFolder)
	srv := server.NewImplementation(imageStore)
	pb.RegisterImageServiceServer(s, srv)
	grpcListener, err := net.Listen("tcp", server.ImagePort)
	if err != nil {
		log.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	go listen(s, grpcListener)
	wg.Wait()
}

func listen(s *grpc.Server, grpcListener net.Listener) {
	if err := s.Serve(grpcListener); err != nil {
		log.Fatal(err)
	}
}
