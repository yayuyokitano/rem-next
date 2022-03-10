package reminteraction

import "errors"

type OptionType int

const (
	oSubCommand      OptionType = 1
	oSubCommandGroup            = 2
	oString                     = 3
	oInt                        = 4
	oBool                       = 5
	oUser                       = 6
	oChannel                    = 7
	oRole                       = 8
	oMentionable                = 9
	oNumber                     = 10
	oAttachment                 = 11
)

type ChannelType int

const (
	cGuildText ChannelType = iota
	cDM
	cGuildVoice
	cGroupDM
	cGuildCategory
	cGuildNews
	cGuildStore
	cGuildNewsThread
	cGuildPublicThread
	cGuildPrivateThread
	cGuildStageVoice
)

type InteractionType int

const (
	iChatInput InteractionType = 1
	iUser                      = 2
	iMessage                   = 3
)

type PermissionType int

const (
	pRole PermissionType = 1
	pUser                = 2
)

type OptionChoice struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Permission struct {
	ID         string         `json:"id"`
	Type       PermissionType `json:"type"`
	Permission bool           `json:"permission"`
}

type Option struct {
	Type         OptionType     `json:"type"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	Required     bool           `json:"required"`
	Choices      []OptionChoice `json:"choices"`
	Options      []Option       `json:"options"`
	ChannelTypes []ChannelType  `json:"channel_types"`
	MinValue     float64        `json:"min_value"`
	MaxValue     float64        `json:"max_value"`
	Autocomplete bool           `json:"autocomplete"`
}

type interactionStructure struct {
	Description string          `json:"description"`
	Options     []Option        `json:"options"`
	Type        InteractionType `json:"type"`
}

type Interaction struct {
	interactionStructure
	Name               string `json:"name"`
	DefaultInteraction bool   `json:"default_interaction"`
}

var interactions = map[string]interactionStructure{
	"level": {
		Description: "Show level or level leaderboard related things.",
		Options: []Option{
			{
				Type:        oSubCommand,
				Name:        "display",
				Description: "Display the level of a user.",
				Options: []Option{
					{
						Type:        oUser,
						Name:        "user",
						Description: "The user to show the level of, defaults to yourself.",
					},
				},
			},
		},
		Type: iChatInput,
	},
	"test": {
		Description: "Test interaction.",
		Options:     []Option{},
		Type:        iChatInput,
	},
}

func createInteraction(name string, defaultPermission bool) (interaction Interaction, err error) {
	structure, ok := interactions[name]
	if !ok {
		err = errors.New("interaction not found")
		return
	}

	interaction = Interaction{
		Name:                 name,
		DefaultInteraction:   defaultPermission,
		interactionStructure: structure,
	}
	return

}
