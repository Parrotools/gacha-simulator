package main

import (
	"flag"
	"log"

	"gacha-simulator/internal/database"
	"gacha-simulator/internal/grpcservice"
	"gacha-simulator/internal/router"
)

func main() {
	mode := flag.String("mode", "all", "Server run mode: all, management, game")
	port := flag.String("port", "", "Server HTTP port (defaults: 8080 for all/management, 8081 for game)")
	flag.Parse()

	switch *mode {
	case "management":
		if err := database.InitDB(); err != nil {
			log.Fatalf("Failed to initialize database: %v", err)
		}
		go grpcservice.StartGRPCServer(":50051")
		r := router.SetupManagementEngine()
		p := ":8080"
		if *port != "" {
			p = ":" + *port
		}
		log.Printf("Starting Management Server on %s (gRPC on :50051)...", p)
		if err := r.Run(p); err != nil {
			log.Fatalf("Management Server failed: %v", err)
		}
	case "game":
		if err := database.InitDB(); err != nil {
			log.Fatalf("Failed to initialize database: %v", err)
		}
		go grpcservice.StartGRPCConfigClient("localhost:50051")
		r := router.SetupGameEngine()
		p := ":8081"
		if *port != "" {
			p = ":" + *port
		}
		log.Printf("Starting Game Server on %s (gRPC connecting to localhost:50051)...", p)
		if err := r.Run(p); err != nil {
			log.Fatalf("Game Server failed: %v", err)
		}
	default:
		if err := database.InitDB(); err != nil {
			log.Fatalf("Failed to initialize database: %v", err)
		}
		go grpcservice.StartGRPCServer(":50051")
		go grpcservice.StartGRPCConfigClient("localhost:50051")
		r := router.SetupRouter()
		p := ":8080"
		if *port != "" {
			p = ":" + *port
		}
		log.Printf("Starting Combined Server on %s (gRPC on :50051)...", p)
		if err := r.Run(p); err != nil {
			log.Fatalf("Combined Server failed: %v", err)
		}
	}
}
