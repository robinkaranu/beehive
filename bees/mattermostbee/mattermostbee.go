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
	"github.com/davecgh/go-spew/spew"
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
	// TODO: perhaps use cache2go
	userCache       map[string]mattermost.User

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

func (mod *MattermostBee) UserById(userId string) mattermost.User {
	v, ok := mod.userCache[userId]
	// bypass request if it has ever made before by this bee
	if ok {
	  return v
	}

	// set an etag
	u, _ := uuid.NewV4()
	etag := u.String()

	user := &mattermost.User{}
	var resp *mattermost.Response

	if user, resp = mod.client.GetUser(userId, etag); resp.Error != nil {
		mod.LogErrorf("Failed to fetch user information for id %s. %s %s", userId, resp.Error.Message, resp.Error.DetailedError)
	}

	spew.Dump(user)
	// inject user to cache
	mod.userCache[userId] = *user
	return *user
}

// Run executes the Bee's event loop.
func (mod *MattermostBee) Run(eventChan chan bees.Event) {
	// channel signaling Mattermost connection status
	mod.connectedState = make(chan bool)
	mod.userCache = make(map[string]mattermost.User)
	// set an etag
	u, _ := uuid.NewV4()
	etag := u.String()

	// setup basic api client:
	mod.client = mattermost.NewAPIv4Client(mod.apiUrl)
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
	var err *mattermost.AppError
	mod.webSocketClient, err = mattermost.NewWebSocketClient4(mod.wsUrl, mod.authToken)
	if err != nil {
		mod.LogErrorf("Problem connecting to Websocket API: %s", err)
		mod.LogFatal("Error setting up Websocket connection.")
	}
	// Fetch our bee's user id
	mod.LogDebugf("Using etag %s for caching.", etag)
	var get_me_resp *mattermost.Response
	mod.clientUser, get_me_resp = mod.client.GetMe(etag)
	if resp.Error != nil {
		mod.LogErrorf("Could not fetch user information: %s", get_me_resp.Error.Message)
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
	switch event.Event {
	case mattermost.WEBSOCKET_EVENT_HELLO:
		ev := bees.Event{
			Bee:  mod.Name(),
			Name: "hello",
			Options: []bees.Placeholder{
				{
					Name:  "user_id",
					Type:  "string",
					Value: event.Broadcast.UserId,
				},
				{
					Name:  "user_name",
					Type:  "string",
					Value: mod.UserById(event.Broadcast.UserId).Username,
				},
				{
					Name:  "server_version",
					Type:  "string",
					Value: event.Data["server_version"],
				},
			},
		}
		eventChan <- ev
		//mod.connectedState <- true
	case mattermost.WEBSOCKET_EVENT_POSTED:
		post := mattermost.PostFromJson(strings.NewReader(event.Data["post"].(string)))
		if post == nil {
			mod.LogErrorf("Could not parse json from: %s", event.Data["post"].(string))
			return
		}
		if post.UserId == mod.clientUser.Id {
			mod.LogDebugf("Skipping my own message")
			return
		}
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
					Name:  "user_name",
					Type:  "string",
					Value: mod.UserById(post.UserId).Username,
				},
				{
					Name:  "text",
					Type:  "string",
					Value: post.Message,
				},
			},
		}
		eventChan <- ev
	case mattermost.WEBSOCKET_EVENT_REACTION_ADDED:
		reaction := mattermost.ReactionFromJson(strings.NewReader(event.Data["reaction"].(string)))
		if reaction == nil {
			mod.LogErrorf("Could not parse json from: %s", event.Data["reaction"].(string))
			return
		}
		if reaction.UserId == mod.clientUser.Id {
			mod.LogDebugf("Skipping my own reaction")
			return
		}
		ev := bees.Event{
			Bee:  mod.Name(),
			Name: "reaction_added",
			Options: []bees.Placeholder{
				{
					Name:  "user_id",
					Type:  "string",
					Value: reaction.UserId,
				},
				{
					Name:  "user_name",
					Type:  "string",
					Value: mod.UserById(reaction.UserId).Username,
				},
				{
					Name:  "post_id",
					Type:  "string",
					Value: reaction.PostId,
				},
				{
					Name:  "emoji_name",
					Type:  "string",
					Value: reaction.EmojiName,
				},
				{
					Name:  "create_at",
					Type:  "integer",
					Value: reaction.CreateAt,
				},
			},
		}
		eventChan <- ev
	case mattermost.WEBSOCKET_EVENT_REACTION_REMOVED:
		reaction := mattermost.ReactionFromJson(strings.NewReader(event.Data["reaction"].(string)))
		if reaction == nil {
			mod.LogErrorf("Could not parse json from: %s", event.Data["reaction"].(string))
			return
		}
		if reaction.UserId == mod.clientUser.Id {
			mod.LogDebugf("Skipping my own reaction removal")
			return
		}
		ev := bees.Event{
			Bee:  mod.Name(),
			Name: "reaction_removed",
			Options: []bees.Placeholder{
				{
					Name:  "user_id",
					Type:  "string",
					Value: reaction.UserId,
				},
				{
					Name:  "user_name",
					Type:  "string",
					Value: mod.UserById(reaction.UserId).Username,
				},
				{
					Name:  "post_id",
					Type:  "string",
					Value: reaction.PostId,
				},
				{
					Name:  "emoji_name",
					Type:  "string",
					Value: reaction.EmojiName,
				},
			},
		}
		eventChan <- ev
	case mattermost.WEBSOCKET_EVENT_STATUS_CHANGE:
		//statusChange := mattermost.StatusFromJson(strings.NewReader(event.Data["status_change"].(string)))
		//spew.Dump(statusChange)
		//if statusChange == nil {
		//	mod.LogErrorf("Could not parse json from: %s", event.Data["status_change"].(string))
		//	return
		//}
		ev := bees.Event{
			Bee:  mod.Name(),
			Name: "status_change",
			Options: []bees.Placeholder{
				{
					Name:  "user_id",
					Type:  "string",
					Value: event.Data["user_id"],
				},
				{
					Name:  "user_name",
					Type:  "string",
					Value: mod.UserById(event.Data["user_id"].(string)).Username,
				},
				{
					Name:  "status",
					Type:  "string",
					Value: event.Data["status"],
				},
			},
		}
		eventChan <- ev
	case mattermost.WEBSOCKET_EVENT_USER_ADDED:
		ev := bees.Event{
			Bee:  mod.Name(),
			Name: "user_added",
			Options: []bees.Placeholder{
				{
					Name:  "user_id",
					Type:  "string",
					Value: event.Data["user_id"],
				},
				{
					Name:  "user_name",
					Type:  "string",
					Value: mod.UserById(event.Data["user_id"].(string)).Username,
				},
				{
					Name:  "channel_id",
					Type:  "string",
					Value: event.Broadcast.ChannelId,
				},
				{
					Name:  "team_id",
					Type:  "string",
					Value: event.Data["team_id"],
				},
			},
		}
		eventChan <- ev
	case mattermost.WEBSOCKET_EVENT_USER_REMOVED:
		ev := bees.Event{
			Bee:  mod.Name(),
			Name: "user_removed",
			Options: []bees.Placeholder{
				{
					Name:  "user_id",
					Type:  "string",
					Value: event.Data["user_id"],
				},
				{
					Name:  "user_name",
					Type:  "string",
					Value: mod.UserById(event.Data["user_id"].(string)).Username,
				},
				{
					Name:  "channel_id",
					Type:  "string",
					Value: event.Broadcast.ChannelId,
				},
				{
					Name:  "remover_id",
					Type:  "string",
					Value: event.Data["remover_id"],
				},
			},
		}
		eventChan <- ev
	default:
		mod.LogDebugf("Websocket event of type %s is not being handled", event.Event)
		spew.Dump(event)
	}
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
