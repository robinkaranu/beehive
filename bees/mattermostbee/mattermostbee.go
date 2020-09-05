/*
 *    Copyright (C) 2014-2019 Christian Muehlhaeuser
 *
 *    This program is free software: you can redistribute it and/or modify
 *    it under the terms of the GNU Affero General Public License as published
 *    by the Free Software Foundation, either version 3 of the License, or
 *    (at your option) any later version.
 *
 *    This program is distributed in the hope that it will be useful,
 *    but WITHOUT ANY WARRANTY; without even the implied warranty of
 *    MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *    GNU Affero General Public License for more details.
 *
 *    You should have received a copy of the GNU Affero General Public License
 *    along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 *    Authors:
 *      krobin
 */

// Package Mattermostbee is a Bee that can connect to an Mattermost server.
package mattermostbee

import (
	"strings"

	mattermost "github.com/mattermost/mattermost-server/v5/model"
	uuid "github.com/nu7hatch/gouuid"

	"github.com/muesli/beehive/bees"
)

// MattermostBee is a Bee that can connect to an Mattermost server.
type MattermostBee struct {
	bees.Bee

	// channel signaling Mattermost connection status
	connectedState chan bool

	client          *mattermost.Client4
	webSocketClient *mattermost.WebSocketClient
	clientUser      *mattermost.User
	channels        []string

	apiUrl    string
	wsUrl     string
	authToken string
	name      string // Not used right now
	teamName  string // Not used right now
}

// Action triggers the action passed to it.
func (mod *MattermostBee) Action(action bees.Action) []bees.Placeholder {
	outs := []bees.Placeholder{}

	// todo: handle more actions, like join/part
	switch action.Name {
	case "send":
		post := &mattermost.Post{}
		action.Options.Bind("channel_id", &post.ChannelId)
		action.Options.Bind("text", &post.Message)
		action.Options.Bind("parent_id", &post.RootId)

		if _, resp := mod.client.CreatePost(post); resp.Error != nil {
			mod.LogErrorf("Failed to send mattermost message to channel %s. %s %s", post.ChannelId, resp.Error.Message, resp.Error.DetailedError)
		}
	default:
		panic("Unknown action triggered in " + mod.Name() + ": " + action.Name)
	}

	return outs
}

// Run executes the Bee's event loop.
func (mod *MattermostBee) Run(eventChan chan bees.Event) {
	// channel signaling Mattermost connection status
	mod.connectedState = make(chan bool)
	// set an etag
	u, _ := uuid.NewV4()
	etag := u.String()

	// setup basic api client:
	mod.client = mattermost.NewAPIv4Client(mod.apiUrl)
	// TODO: loginfoo (mail + password login -> token)
	mod.client.SetToken(mod.authToken)

	// test basic connection. ClientConfig in old format contains server version
	props, resp := mod.client.GetOldClientConfig("")
	if resp.Error != nil {
		mod.LogErrorf("Problem connecting to Mattermost API: %s", resp.Error.Message)
		mod.LogFatal("Couldn't connect to Mattermost.")
	}
	mod.LogDebugf("Server detected and is running version %s", props["Version"])
	mod.LogDebugf("Client logged in. Auth Token: %s.", mod.client.AuthToken)

	// setup websocket client:
	var err error
	mod.webSocketClient, err = mattermost.NewWebSocketClient4(mod.wsUrl, mod.authToken)
	if err != nil {
		mod.LogErrorf("Problem connecting to Websocket API: %s", err.Message)
		mod.LogFatal("Error setting up Websocket connection.")
	}

	// Fetch our bee's user id
	mod.LogDebugf("Using etag %s for caching.", etag)
	var resp *mattermost.Response
	mod.clientUser, resp = mod.client.GetMe(etag)
	if resp.Error != nil {
		mod.LogErrorf("Could not fetch user information: %s", resp.Error.Message)
		mod.LogFatal("Could not get own user id, loop protection not possible")
	}

	mod.webSocketClient.Listen()
	// TODO: when may be connected
	//mod.connectedState <- true

	// The bees event loop (dispatching websocket events to bees event chan)
	for {
		select {
		case resp := <-mod.webSocketClient.EventChannel:
			mod.HandleWebSocketResponse(resp, eventChan)
		}
	}
}

// HandleWebSocketResponse creates a bees.Event for (certain) incoming websocket events
func (mod *MattermostBee) HandleWebSocketResponse(event *mattermost.WebSocketEvent, eventChan chan bees.Event) {
	// Lets only reponded to messaged posted events
	if event.Event != mattermost.WEBSOCKET_EVENT_POSTED {
		return
	}

	post := mattermost.PostFromJson(strings.NewReader(event.Data["post"].(string)))
	if post == nil {
		mod.LogErrorf("Could not parse json from: %s", event.Data["post"].(string))
		return
	}
	if post.UserId == mod.clientUser.Id {
		mod.LogDebug("Skipping my own message")
		return
	}
	// TODO: emit signals for more actions. see TODOs in factory
	ev := bees.Event{
		Bee:  mod.Name(),
		Name: "message",
		Options: []bees.Placeholder{
			{
				Name:  "id",
				Type:  "string",
				Value: post.Id,
			},
			{
				Name:  "channel_id",
				Type:  "string",
				Value: post.ChannelId,
			},
			{
				Name:  "user_id",
				Type:  "string",
				Value: post.UserId,
			},
			{
				Name:  "text",
				Type:  "string",
				Value: post.Message,
			},
		},
	}
	eventChan <- ev
}

// ReloadOptions parses the config options and initializes the Bee.
func (mod *MattermostBee) ReloadOptions(options bees.BeeOptions) {
	mod.SetOptions(options)
	options.Bind("api_url", &mod.apiUrl)
	options.Bind("ws_url", &mod.wsUrl)
	options.Bind("auth_token", &mod.authToken)
	options.Bind("name", &mod.name)
	options.Bind("team_name", &mod.teamName)
	options.Bind("channels", &mod.channels)
	mod.ContextSet("channels", &mod.channels)
}
