package server

import (
	"context"
    "time"
	"fmt"
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
    BidCh       chan *BidEvent
    Mutex       sync.Mutex
    Closed      bool
}

type BidEvent struct {
    UserID   string
    Username string
    ProductID string
    Amount   float64
}


type AuctionServer struct {
	pb.UnimplementedAuctionServiceServer
	Rooms       map[string]*Room
    UserRoom    map[string]string
	Mutex       sync.Mutex
}

func NewAuctionServer() *AuctionServer {
	return &AuctionServer{
		Rooms: make(map[string]*Room),
        UserRoom: make(map[string]string),
	}
}

func (s *AuctionServer) AddProduct(ctx context.Context, req *pb.Product) (*pb.ProductResponse, error) {
    s.Mutex.Lock()
    defer s.Mutex.Unlock()

    if _, exists := s.Rooms[req.ProductId]; exists {
        return &pb.ProductResponse{Success: false, Message: "Product already exists"}, nil
    }

	room := &Room{
		Product:    req,
		Bidders:    make(map[string]*Bidder),
		BidCh:      make(chan *BidEvent, 100),
		HighestBid: req.StartingPrice,
	}	

	s.Rooms[req.ProductId] = room

    auctionDuration := 10
	go s.handleRoomBids(room, auctionDuration)

    return &pb.ProductResponse{Success: true, Message: "Product added"}, nil
}

func (s *AuctionServer) handleRoomBids(room *Room, auctionDurationMinutes int) {
    go func() {
        time.Sleep(time.Duration(auctionDurationMinutes) * time.Minute)

        room.Mutex.Lock()
        room.Closed = true

        finalUpdate := &pb.BidUpdate{
            HighestBidderName: room.HighestUser,
            HighestAmount:     room.HighestBid,
            ProductId:         room.Product.ProductId,
            AuctionClosed:     true,
        }
        for _, bidder := range room.Bidders {
            select {
            case bidder.UpdateCh <- finalUpdate:
            default:
            }
        }
        for _, bidder := range room.Bidders {
            close(bidder.UpdateCh)
        }
        room.Mutex.Unlock()

        close(room.BidCh)
    }()

    for event := range room.BidCh {
        update := &pb.BidUpdate{
            HighestBidderName: event.Username,
            HighestAmount:     event.Amount,
            ProductId:         event.ProductID,
            AuctionClosed:     false,
        }

        room.Mutex.Lock()
        for _, bidder := range room.Bidders {
            select {
            case bidder.UpdateCh <- update:
            default:
            }
        }
        room.Mutex.Unlock()
    }
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

func (s *AuctionServer) Join(req *pb.JoinRequest, stream pb.AuctionService_JoinServer) error {
    s.Mutex.Lock()
    if existingRoom, ok := s.UserRoom[req.UserId]; ok {
        s.Mutex.Unlock()
        return fmt.Errorf("user already joined room %s", existingRoom)
    }

    room, exists := s.Rooms[req.ProductId]
    if !exists {
        s.Mutex.Unlock()
        return fmt.Errorf("room not found")
    }

    s.UserRoom[req.UserId] = req.ProductId
    s.Mutex.Unlock()

    bidder := &Bidder{
        UserID:   req.UserId,
        Username: req.Username,
        UpdateCh: make(chan *pb.BidUpdate, 100),
    }

    room.Mutex.Lock()
    room.Bidders[req.UserId] = bidder
    room.Mutex.Unlock()

    cleanup := func() {
        room.Mutex.Lock()
        delete(room.Bidders, req.UserId)
        room.Mutex.Unlock()

        s.Mutex.Lock()
        delete(s.UserRoom, req.UserId)
        s.Mutex.Unlock()
    }

    defer cleanup()

    for {
        select {
        case update, ok := <-bidder.UpdateCh:
            if !ok {
                return nil
            }
            if err := stream.Send(update); err != nil {
                return err
            }
        case <-stream.Context().Done():
            return nil
        }
    }
}



func (s *AuctionServer) PlaceBid(ctx context.Context, req *pb.BidRequest) (*pb.BidResponse, error) {
    s.Mutex.Lock()
    room, exists := s.Rooms[req.ProductId]
    s.Mutex.Unlock()

    if !exists {
        return &pb.BidResponse{Success: false, Message: "Room not found"}, nil
    }

    room.Mutex.Lock()
    defer room.Mutex.Unlock()

    if _, ok := room.Bidders[req.UserId]; !ok {
        return &pb.BidResponse{Success: false, Message: "User has not joined this room"}, nil
    }

    if room.Closed {
        return &pb.BidResponse{Success: false, Message: "Auction is closed"}, nil
    }

    if req.Amount <= room.HighestBid {
        return &pb.BidResponse{Success: false, Message: fmt.Sprintf("Bid too low, current highest bid: %.2f", room.HighestBid)}, nil
    }

    room.HighestBid = req.Amount
    room.HighestUser = req.UserId

    username := req.UserId
    if b, ok := room.Bidders[req.UserId]; ok {
        username = b.Username
    }

    room.BidCh <- &BidEvent{
        UserID:    req.UserId,
        Username:  username,
        ProductID: req.ProductId,
        Amount:    req.Amount,
    }

    return &pb.BidResponse{Success: true, Message: "Bid placed successfully"}, nil
}
