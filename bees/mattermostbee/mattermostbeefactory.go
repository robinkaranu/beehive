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

package mattermostbee

import (
	"github.com/muesli/beehive/bees"
)

// MattermostBeeFactory is a factory for MattermostBees.
type MattermostBeeFactory struct {
	bees.BeeFactory
}

// New returns a new Bee instance configured with the supplied options.
func (factory *MattermostBeeFactory) New(name, description string, options bees.BeeOptions) bees.BeeInterface {
	bee := MattermostBee{
		Bee: bees.NewBee(name, factory.ID(), description, options),
	}
	bee.ReloadOptions(options)

	return &bee
}

// ID returns the ID of this Bee.
func (factory *MattermostBeeFactory) ID() string {
	return "mattermostbee"
}

// Name returns the name of this Bee.
func (factory *MattermostBeeFactory) Name() string {
	return "Mattermost"
}

// Description returns the description of this Bee.
func (factory *MattermostBeeFactory) Description() string {
	return "Connects to Mattermost Websocket API, receives and sends messages as a bot user"
}

// Image returns the filename of an image for this Bee.
func (factory *MattermostBeeFactory) Image() string {
	// TODO: have an image
	return factory.ID() + ".png"
}

// LogoColor returns the preferred logo background color (used by the admin interface).
func (factory *MattermostBeeFactory) LogoColor() string {
	return "#cf3476"
}

// Options returns the options available to configure this Bee.
func (factory *MattermostBeeFactory) Options() []bees.BeeOptionDescriptor {
	opts := []bees.BeeOptionDescriptor{
		{
			Name:        "api_url",
			Description: "HTTPs URL of Mattermost server, eg: https://chat.foo.de:1234",
			Type:        "url",
			Mandatory:   true,
		},
		{
			Name:        "ws_url",
			Description: "WSs URL of Mattermost server, eg: wss://chat.foo.de:5678",
			Type:        "url",
			Mandatory:   true,
		},
		{
			Name:        "name",
			Description: "unimplemented: Nickname to use for Mattermost bot user",
			Type:        "string",
			Default:     "beehive",
		},
		{
			Name:        "team_name",
			Description: "unimplemented: Team (name) to use for Mattermost bot user",
			Type:        "string",
		},
		{
			Name:        "auth_token",
			Description: "API-Token to use to connect to the Mattermost server",
			Type:        "password",
			Mandatory:   true,
		},
		{
			Name:        "channels",
			Description: "unimplemented: Which channels to join",
			Type:        "[]string",
			Mandatory:   true,
		},
	}
	return opts
}

// States returns the state values provided by this Bee.
func (factory *MattermostBeeFactory) States() []bees.StateDescriptor {
	opts := []bees.StateDescriptor{
		{
			Name:        "connected",
			Description: "Whether this bee is currently connected to a Mattermost server",
			Type:        "bool",
		},
		{
			Name:        "channels",
			Description: "unimplemented: Which channels this bee is currently connected to",
			Type:        "[]string",
		},
	}
	return opts
}

// Events describes the available events provided by this Bee.
func (factory *MattermostBeeFactory) Events() []bees.EventDescriptor {
	events := []bees.EventDescriptor{
		{
			Namespace:   factory.Name(),
			Name:        "message",
			Description: "A message was received over Mattermost",
			Options: []bees.PlaceholderDescriptor{
				{
					Name:        "text",
					Description: "The message that was received",
					Type:        "string",
				},
				{
					Name:        "id",
					Description: "The received message id",
					Type:        "string",
				},
				{
					Name:        "user_id",
					Description: "The user id of the message author",
					Type:        "string",
				},
				{
					Name:        "channel_id",
					Description: "The channel_id the message was sent on",
					Type:        "string",
				},
				// TODO: also output channel name
				// TODO: channel-type
			},
		},
		{
			Namespace:   factory.Name(),
			Name:        "reaction_added",
			Description: "An emoji reaction was added over Mattermost",
			Options: []bees.PlaceholderDescriptor{
				{
					Name:        "emoji_name",
					Description: "Text name of the emoji reaction",
					Type:        "string",
				},
				{
					Name:        "post_id",
					Description: "The id of the post which was reacted to",
					Type:        "string",
				},
				{
					Name:        "user_id",
					Description: "The user id of the reactions author",
					Type:        "string",
				},
				{
					Name:        "create_at",
					Description: "The time of the reaction ",
					Type:        "integer",
				},
			},
		},
		{
			Namespace:   factory.Name(),
			Name:        "reaction_removed",
			Description: "An emoji reaction was removed over Mattermost",
			Options: []bees.PlaceholderDescriptor{
				{
					Name:        "emoji_name",
					Description: "Text name of the emoji reaction",
					Type:        "string",
				},
				{
					Name:        "post_id",
					Description: "The id of the post which was reacted to",
					Type:        "string",
				},
				{
					Name:        "user_id",
					Description: "The user id of the reactions author",
					Type:        "string",
				},
			},
		},
		{
			Namespace:   factory.Name(),
			Name:        "status_change",
			Description: "An user status changed on Mattermost",
			Options: []bees.PlaceholderDescriptor{
				{
					Name:        "user_id",
					Description: "The user id which status changed",
					Type:        "string",
				},
				{
					Name:        "status",
					Description: "The new user status",
					Type:        "string",
				},
			},
		},
		{
			Namespace:   factory.Name(),
			Name:        "user_added",
			Description: "An user was added to a channel on Mattermost",
			Options: []bees.PlaceholderDescriptor{
				{
					Name:        "user_id",
					Description: "The user id which status changed",
					Type:        "string",
				},
				{
					Name:        "channel_id",
					Description: "The new user status",
					Type:        "string",
				},
				{
					Name:        "team_id",
					Description: "The team id",
					Type:        "string",
				},
			},
		},
		{
			Namespace:   factory.Name(),
			Name:        "user_removed",
			Description: "An user was removed from a channel on Mattermost",
			Options: []bees.PlaceholderDescriptor{
				{
					Name:        "user_id",
					Description: "The user id",
					Type:        "string",
				},
				{
					Name:        "channel_id",
					Description: "The new user status",
					Type:        "string",
				},
				{
					Name:        "remover id",
					Description: "The remover id",
					Type:        "string",
				},
			},
		},
		// TODO: post was pinned/unpinned
	}
	return events
}

// Actions describes the available actions provided by this Bee.
func (factory *MattermostBeeFactory) Actions() []bees.ActionDescriptor {
	actions := []bees.ActionDescriptor{
		{
			Namespace:   factory.Name(),
			Name:        "send",
			Description: "Sends a message to Mattermost",
			Options: []bees.PlaceholderDescriptor{
				{
					Name:        "channel_id",
					Description: "Which channel id to send the message to",
					Type:        "string",
					Mandatory:   true,
				},
				{
					Name:        "parent_id",
					Description: "The parent message id (optional)",
					Type:        "string",
				},
				{
					Name:        "text",
					Description: "Content of the message",
					Type:        "string",
					Mandatory:   true,
				},
			},
		},
		// TODO: reaction_removed
	}
	return actions
}

func init() {
	f := MattermostBeeFactory{}
	bees.RegisterFactory(&f)
}
