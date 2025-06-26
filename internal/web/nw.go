// internal/web/nw.go

package web

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"tritontube/internal/proto"

	"google.golang.org/grpc"
)

type nodeInfo struct {
	address string
	hash    uint64
	client  proto.VideoContentServiceClient
}

//	type NetworkVideoContentService struct {
//		mu       sync.RWMutex
//		nodes    []nodeInfo
//		hashRing []uint64
//	}
type NetworkVideoContentService struct {
	mu       sync.RWMutex
	nodes    []nodeInfo
	hashRing []uint64

	proto.UnimplementedVideoContentAdminServiceServer
}

var _ VideoContentService = (*NetworkVideoContentService)(nil)
var _ proto.VideoContentAdminServiceServer = (*NetworkVideoContentService)(nil)

func NewNetworkVideoContentService(addresses []string) (*NetworkVideoContentService, error) {
	nvcs := &NetworkVideoContentService{}
	for _, addr := range addresses {
		if err := nvcs.addNode(addr); err != nil {
			return nil, err
		}
	}
	nvcs.rebuildRing()
	return nvcs, nil
}

func hashStringToUint64(s string) uint64 {
	sum := sha256.Sum256([]byte(s))
	return binary.BigEndian.Uint64(sum[:8])
}

func (n *NetworkVideoContentService) addNode(addr string) error {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("failed to connect to node %s: %w", addr, err)
	}
	client := proto.NewVideoContentServiceClient(conn)
	hash := hashStringToUint64(addr)
	n.nodes = append(n.nodes, nodeInfo{address: addr, hash: hash, client: client})
	return nil
}

func (n *NetworkVideoContentService) rebuildRing() {
	sort.Slice(n.nodes, func(i, j int) bool {
		return n.nodes[i].hash < n.nodes[j].hash
	})
	n.hashRing = make([]uint64, len(n.nodes))
	for i, node := range n.nodes {
		n.hashRing[i] = node.hash
	}
}

func (n *NetworkVideoContentService) getNodeForKey(key string) *nodeInfo {
	h := hashStringToUint64(key)
	for _, node := range n.nodes {
		if h <= node.hash {
			return &node
		}
	}
	if len(n.nodes) > 0 {
		return &n.nodes[0] // wrap around
	}
	return nil
}

func (n *NetworkVideoContentService) Write(videoId, filename string, data []byte) error {
	key := fmt.Sprintf("%s/%s", videoId, filename)
	node := n.getNodeForKey(key)
	if node == nil {
		return errors.New("no storage nodes available")
	}
	_, err := node.client.Write(context.Background(), &proto.WriteRequest{
		VideoId:  videoId,
		Filename: filename,
		Data:     data,
	})
	return err
}

func (n *NetworkVideoContentService) Read(videoId, filename string) ([]byte, error) {
	key := fmt.Sprintf("%s/%s", videoId, filename)
	node := n.getNodeForKey(key)
	if node == nil {
		return nil, errors.New("no storage nodes available")
	}
	resp, err := node.client.Read(context.Background(), &proto.ReadRequest{
		VideoId:  videoId,
		Filename: filename,
	})
	if err != nil {
		return nil, err
	}
	return resp.Data, nil
}

func (n *NetworkVideoContentService) ListNodes(ctx context.Context, _ *proto.ListNodesRequest) (*proto.ListNodesResponse, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	nodes := make([]string, len(n.nodes))
	for i, node := range n.nodes {
		nodes[i] = node.address
	}
	return &proto.ListNodesResponse{Nodes: nodes}, nil
}

func (n *NetworkVideoContentService) AddNode(ctx context.Context, req *proto.AddNodeRequest) (*proto.AddNodeResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	oldNodes := append([]nodeInfo(nil), n.nodes...)
	if err := n.addNode(req.NodeAddress); err != nil {
		return nil, err
	}
	n.rebuildRing()

	migrated := 0
	newNode := n.getNodeForKey(req.NodeAddress)
	internalCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	for _, node := range oldNodes {
		resp, err := node.client.ListKeys(internalCtx, &proto.ListKeysRequest{})
		if err != nil {
			continue
		}
		for _, key := range resp.Keys {
			parts := strings.SplitN(key, "/", 2)
			if len(parts) != 2 {
				continue
			}
			videoId, filename := parts[0], parts[1]
			target := n.getNodeForKey(key)
			if target.address == newNode.address {
				readResp, err := node.client.Read(ctx, &proto.ReadRequest{
					VideoId:  videoId,
					Filename: filename,
				})
				if err != nil {
					continue
				}
				_, _ = newNode.client.Write(ctx, &proto.WriteRequest{
					VideoId:  videoId,
					Filename: filename,
					Data:     readResp.Data,
				})
				_, _ = node.client.Delete(ctx, &proto.DeleteRequest{
					VideoId:  videoId,
					Filename: filename,
				})
				migrated++
			}
		}
	}
	return &proto.AddNodeResponse{MigratedFileCount: int32(migrated)}, nil
}

func (n *NetworkVideoContentService) RemoveNode(ctx context.Context, req *proto.RemoveNodeRequest) (*proto.RemoveNodeResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	index := -1
	for i, node := range n.nodes {
		if node.address == req.NodeAddress {
			index = i
			break
		}
	}
	if index == -1 {
		return nil, fmt.Errorf("node not found")
	}

	removed := n.nodes[index]
	n.nodes = append(n.nodes[:index], n.nodes[index+1:]...)
	n.rebuildRing()

	resp, err := removed.client.ListKeys(ctx, &proto.ListKeysRequest{})
	if err != nil {
		return nil, err
	}

	migrated := 0
	for _, key := range resp.Keys {
		parts := strings.SplitN(key, "/", 2)
		if len(parts) != 2 {
			continue
		}
		videoId, filename := parts[0], parts[1]
		readResp, err := removed.client.Read(ctx, &proto.ReadRequest{
			VideoId:  videoId,
			Filename: filename,
		})
		if err != nil {
			continue
		}
		target := n.getNodeForKey(key)
		_, _ = target.client.Write(ctx, &proto.WriteRequest{
			VideoId:  videoId,
			Filename: filename,
			Data:     readResp.Data,
		})
		_, _ = removed.client.Delete(ctx, &proto.DeleteRequest{
			VideoId:  videoId,
			Filename: filename,
		})
		migrated++
	}
	return &proto.RemoveNodeResponse{MigratedFileCount: int32(migrated)}, nil
}
