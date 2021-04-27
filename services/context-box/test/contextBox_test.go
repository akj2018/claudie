package test

import (
	"io/ioutil"
	"log"
	"testing"

	"github.com/Berops/platform/ports"
	"github.com/Berops/platform/proto/pb"
	cbox "github.com/Berops/platform/services/context-box/client"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestGetConfig(t *testing.T) {
	//Create connection to Context-box
	cc, err := grpc.Dial(ports.ContextBoxPort, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to server: %v", err)
	}
	defer func() {
		err := cc.Close()
		require.NoError(t, err)
	}()

	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)

	res, err := cbox.GetConfig(c)
	require.NoError(t, err)
	t.Logf("ID                       Name\n")
	for _, c := range res.GetConfigs() {
		t.Log(c.GetId(), c.GetName(), c.GetDesiredState(), c.CurrentState)
	}
}

func TestSaveConfig(t *testing.T) {
	//Create connection to Context-box
	cc, err := grpc.Dial(ports.ContextBoxPort, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to server: %v", err)
	}
	defer func() {
		err := cc.Close()
		require.NoError(t, err)
	}()

	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)

	manifest, errR := ioutil.ReadFile("manifest.yaml") //this is manifest from this test file
	if errR != nil {
		log.Fatalln(errR)
	}

	err = cbox.SaveConfig(c, &pb.SaveConfigRequest{
		Config: &pb.Config{
			Id:           "6087d18da56a5b590dd7f289",
			Name:         "timestamp test",
			Manifest:     string(manifest),
			DesiredState: &pb.Project{Name: "This is desiredState name"},
			CurrentState: &pb.Project{Name: "This is currentState name"},
		},
		IsFromScheduler: false,
	})
	require.NoError(t, err)
}

func TestDeleteConfig(t *testing.T) {
	//Create connection to Context-box
	cc, err := grpc.Dial(ports.ContextBoxPort, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("could not connect to server: %v", err)
	}
	defer func() {
		err := cc.Close()
		require.NoError(t, err)
	}()

	// Creating the client
	c := pb.NewContextBoxServiceClient(cc)

	err = cbox.DeleteConfig(c, "6049f856a0cf8c4d391e6f57")
	require.NoError(t, err)
}
