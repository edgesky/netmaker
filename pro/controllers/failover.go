package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	controller "github.com/gravitl/netmaker/controllers"
	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/mq"
	proLogic "github.com/gravitl/netmaker/pro/logic"
	"golang.org/x/exp/slog"
)

type FailOverMeReq struct {
	PeerPubKey string `json:"peer_pub_key"`
}

// RelayHandlers - handle Pro Relays
func FailOverHandler(r *mux.Router) {
	r.HandleFunc("/api/v1/host/{hostid}/failoverme", controller.Authorize(true, false, "host", http.HandlerFunc(failOverME))).Methods(http.MethodPost)
}

// swagger:route POST /api/host/failOverME host failOverME
//
// Create a relay.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: nodeResponse
func failOverME(w http.ResponseWriter, r *http.Request) {
	var params = mux.Vars(r)
	hostid := params["hostid"]
	// confirm host exists
	host, err := logic.GetHost(hostid)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "failed to get host:", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	/*
		1. Set On victimNode that needs failedOver to reach - the FailOver and FailedOverBY
		2. On the Node that needs to reach Victim Node, add to failovered Peers
	*/
	var failOverReq FailOverMeReq
	err = json.NewDecoder(r.Body).Decode(&failOverReq)
	if err != nil {
		logger.Log(0, r.Header.Get("user"), "error decoding request body: ", err.Error())
		logic.ReturnErrorResponse(w, r, logic.FormatError(err, "badrequest"))
		return
	}
	var sendPeerUpdate bool
	allNodes, err := logic.GetAllNodes()
	if err != nil {
		slog.Error("failed to get all nodes", "error", err)
	}
	logic.GetHost()
	for _, nodeID := range host.Nodes {
		node, err := logic.GetNodeByID(nodeID)
		if err != nil {
			slog.Error("couldn't find node", "id", nodeID, "error", err)
			continue
		}
		if node.IsRelayed {
			continue
		}
		// get auto relay Host in this network
		failOverNode, err := proLogic.GetFailOverNode(node.Network, allNodes)
		if err != nil {
			slog.Error("auto relay not found", "network", node.Network)
			continue
		}
		peerHost, err := logic.GetHostByPubKey(failOverReq.PeerPubKey)
		if err != nil {

		}

		err = proLogic.SetFailOverCtx(failOverNode, node, models.Node{})
		if err != nil {
			slog.Error("failed to create relay:", "id", node.ID.String(),
				"network", node.Network, "error", err)
			continue
		}
		slog.Info("[auto-relay] created relay on node", "node", node.ID.String(), "network", node.Network)
		sendPeerUpdate = true
	}

	if sendPeerUpdate {
		go mq.PublishPeerUpdate()
	}

	w.Header().Set("Content-Type", "application/json")
	logic.ReturnSuccessResponse(w, r, "relayed successfully")
}
