package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Berops/platform/proto/pb"
	"github.com/Berops/platform/serializer"
	"github.com/Berops/platform/urls"

	"google.golang.org/grpc"
)

func main() {
	cc, err := grpc.Dial(urls.BuilderURL, grpc.WithInsecure()) //connects to a grpc server
	if err != nil {
		log.Fatalf("could not connect to server: %v", err)
	}
	defer cc.Close() //close the connection after response is received

	c := pb.NewBuilderServiceClient(cc)

	project := &pb.Project{}
	// err = serializer.ReadProtobufFromBinaryFile(project, "../../tmp/project.bin") //reads project from binary file and converts it into protobuf
	// if err != nil {
	// 	log.Fatalln("Failed to read project binary file:", err)
	// }

	err = serializer.ReadProtobufFromJSONFile(project, "../../tmp/project.json") //reads project from json file and converts it into protobuf
	if err != nil {
		log.Fatalln("Failed to read project json file:", err)
	}

	build(c, project)
}

func build(c pb.BuilderServiceClient, project *pb.Project) {
	fmt.Println("Starting to do a Unary RPC")
	req := project

	res, err := c.Build(context.Background(), req)
	if err != nil {
		log.Fatalln("error while sending message to Builder", err)
	}
	log.Println("Received message from Builder:", res)
}
