package main

import (
    "log"
    "net"

	"github.com/prakash03445/GoBid/server"

    pb "github.com/prakash03445/GoBidProto/gen/go"
    "google.golang.org/grpc"
)

func main() {
    lis, err := net.Listen("tcp", ":50051")
    if err != nil {
        log.Fatalf("failed to listen: %v", err)
    }

    grpcServer := grpc.NewServer()
    auctionServer := server.NewAuctionServer()

    pb.RegisterAuctionServiceServer(grpcServer, auctionServer)

    log.Println("GoBid gRPC server running on :50051")
    if err := grpcServer.Serve(lis); err != nil {
        log.Fatalf("failed to serve: %v", err)
    }
}
