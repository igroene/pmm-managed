// pmm-managed
// Copyright (C) 2017 Percona LLC
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package handlers

import (
	"time"

	"github.com/golang/protobuf/ptypes"
	api "github.com/percona/pmm/api/agent"
	"github.com/pkg/errors"

	"github.com/percona/pmm-managed/services/agents"
	"github.com/percona/pmm-managed/utils/logger"
)

type AgentServer struct {
}

func (s *AgentServer) Connect(stream api.Agent_ConnectServer) error {
	l := logger.Get(stream.Context())

	// connect request/response
	agentMessage, err := stream.Recv()
	if err != nil {
		return err
	}
	l.Infof("Recv: %T %s.", agentMessage, agentMessage)
	auth := agentMessage.GetAuth()
	if auth == nil {
		return errors.Errorf("Expected AuthRequest, got %T.", agentMessage.Payload)
	}
	serverMessage := &api.ServerMessage{
		Payload: &api.ServerMessage_Auth{
			Auth: &api.AuthResponse{},
		},
	}
	if err = stream.Send(serverMessage); err != nil {
		return err
	}

	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	channel := agents.NewChannel(stream)
	for {
		select {
		case <-stream.Context().Done():
			return nil

		// case exporter := <-s.Store.NewExporters():
		// 	env := []string{
		// 		`DATA_SOURCE_NAME="/"`,
		// 	}
		// 	_, err = conn.SendAndRecv(&agent.ServerMessage_State{
		// 		State: &agent.SetStateRequest{
		// 			AgentProcesses: []*agent.SetStateRequest_AgentProcess{{
		// 				AgentId: exporter.Id,
		// 				Type:    inventory.AgentType_MYSQLD_EXPORTER,
		// 				Args:    nil,
		// 				Env:     env,
		// 				Configs: nil,
		// 			}},
		// 		},
		// 	})
		// 	if err != nil {
		// 		return err
		// 	}

		case <-t.C:
			start := time.Now()
			agentMessage := channel.SendRequest(&api.ServerMessage_Ping{
				Ping: &api.PingRequest{},
			})
			latency := time.Since(start) / 2
			t, err := ptypes.Timestamp(agentMessage.(*api.AgentMessage_Ping).Ping.CurrentTime)
			if err != nil {
				return err
			}
			l.Debugf("Latency: %s. Time drift: %s.", latency, t.Sub(start.Add(latency)))
		}
	}
}

// check interfaces
var (
	_ api.AgentServer = (*AgentServer)(nil)
)