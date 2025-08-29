package server

import (
	"context"
	"sync"

	pb "github.com/prakash03445/GoBidProto/gen/go"
)

type Bidder struct {
	UserID   string
	Username string
	UpdateCh chan *pb.BidUpdate
}

type Room struct {
	Product     *pb.Product
	HighestBid  float64
	HighestUser string
	Bidders     map[string]*Bidder
	Mutex       sync.Mutex
	Closed      bool
}

type AuctionServer struct {
	pb.UnimplementedAuctionServiceServer
	Rooms map[string]*Room
	Mutex sync.Mutex
}

func NewAuctionServer() *AuctionServer {
	return &AuctionServer{
		Rooms: make(map[string]*Room),
	}
}

func (s *AuctionServer) AddProduct(ctx context.Context, req *pb.Product) (*pb.ProductResponse, error) {
    s.Mutex.Lock()
    defer s.Mutex.Unlock()

    if _, exists := s.Rooms[req.ProductId]; exists {
        return &pb.ProductResponse{Success: false, Message: "Product already exists"}, nil
    }

    s.Rooms[req.ProductId] = &Room{
        Product: req,
    }

    return &pb.ProductResponse{Success: true, Message: "Product added"}, nil
}


func (s *AuctionServer) GetProducts(ctx context.Context, _ *pb.Empty) (*pb.ProductListResponse, error) {
    s.Mutex.Lock()
    defer s.Mutex.Unlock()

    products := []*pb.Product{}
    for _, room := range s.Rooms {
        products = append(products, room.Product)
    }

    return &pb.ProductListResponse{Product: products}, nil
}
