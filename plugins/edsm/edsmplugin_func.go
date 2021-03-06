package edsm

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"github.com/dmportella/qilbot/bot"
	"github.com/dmportella/qilbot/logging"
	"strconv"
	"strings"
)

// NewPlugin creates a new instance of EDSMPlugin.
func NewPlugin(qilbot *bot.Qilbot) (plugin *Plugin) {
	const (
		Name        = "EDSM Plugin"
		Description = "Client plugin for Elite Dangerous Star Map web site."
	)

	debugMode := qilbot.InDebugMode()

	plugin = &Plugin{
		Plugin: bot.Plugin{
			Qilbot:      qilbot,
			Name:        Name,
			Description: Description,
			Commands: []bot.CommandInformation{
				{
					Command:     "distance",
					Template:    "distance **sys1** / **sys2**",
					Description: "Uses the coords in EDSM to calculate the distance between the two star systems.",
					Execute: func(s *discordgo.Session, m *discordgo.MessageCreate, commandText string) {
						plugin.distanceCommand(s, m, commandText)
					},
				},
				{
					Command:     "sphere",
					Template:    "sphere **sys1** 14.33ly",
					Description: "Returns a list of systems within a specified distance to specified system.",
					Execute: func(s *discordgo.Session, m *discordgo.MessageCreate, commandText string) {
						plugin.sphereCommand(s, m, commandText)
					},
				},
			},
		},
		api: NewAPIClient(debugMode),
	}

	qilbot.AddPlugin(plugin)

	qilbot.AddCommand(&plugin.Commands[0])
	qilbot.AddCommand(&plugin.Commands[1])

	return
}

func (plugin *Plugin) sphereCommand(s *discordgo.Session, m *discordgo.MessageCreate, commandText string) {
	placeMatches := regexMatchSphereCommand(commandText)

	logging.Trace.Println(placeMatches)

	if len(placeMatches) >= 3 {
		systemName := strings.TrimSpace(placeMatches[1])
		distance := strings.TrimSpace(placeMatches[2])

		logging.Trace.Println("systemname", systemName, "distance", distance)

		s.ChannelTyping(m.ChannelID)

		if sys1, ok1 := plugin.api.GetSystem(systemName); ok1 == nil {
			if systems, ok2 := plugin.getSphereSystems(sys1.Name, distance); ok2 == nil {
				var buffer bytes.Buffer

				header := fmt.Sprintf("Found **%d** systems within **%sly** of **%s**.\r\n", len(systems)-1, distance, sys1.Name)

				buffer.WriteString(header)

				buffer.Write([]byte("```\r\n"))

				for _, sys2 := range systems {
					if sys2.Name == sys1.Name {
						continue
					}

					fmt.Fprintf(&buffer, "%-30s\t\t\t\t%9.2fly\r\n", sys2.Name, calculateDistance(sys1.Coords, sys2.Coords))
				}

				buffer.Write([]byte("```\r\n"))

				if buffer.Len() > 8388608 {
					logging.Trace.Println("msg large", buffer.Len())

					_, _ = s.ChannelMessageSend(m.ChannelID, "Response is too large for discord please narrow you search.")
				} else if buffer.Len() > 2000 {
					logging.Trace.Println("msg attach", buffer.Len())
					reader := bytes.NewReader(buffer.Bytes())
					_, _ = s.ChannelFileSendWithMessage(m.ChannelID, header, "Results.txt", reader)
				} else {
					logging.Trace.Println("msg oke", buffer.Len())
					_, _ = s.ChannelMessageSend(m.ChannelID, buffer.String())
				}
			} else {
				_, _ = s.ChannelMessageSend(m.ChannelID, ok2.Error())
			}
		} else {
			_, _ = s.ChannelMessageSend(m.ChannelID, ok1.Error())
		}
	}
}

func (plugin *Plugin) distanceCommand(s *discordgo.Session, m *discordgo.MessageCreate, commandText string) {
	placeMatches := regexMatchDistanceCommand(commandText)

	if len(placeMatches) >= 3 {
		sys1 := strings.TrimSpace(placeMatches[1])
		sys2 := strings.TrimSpace(placeMatches[2])

		s.ChannelTyping(m.ChannelID)

		if distance, err := plugin.getDistanceBetweenTwoSystems(sys1, sys2); err == nil {
			_, _ = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("The distance between **%s** and **%s** is **%.2fly**.", sys1, sys2, distance))
		} else {
			logging.Trace.Println(err)
			_, _ = s.ChannelMessageSend(m.ChannelID, err.Error())
		}
	} else {
		_, _ = s.ChannelMessageSend(m.ChannelID, "Please give me two places, format: distance **A** / **B**")
	}
}

func (plugin *Plugin) getDistanceBetweenTwoSystems(systemName1 string, systemName2 string) (distance float64, err error) {
	if sys1, ok1 := plugin.api.GetSystem(systemName1); ok1 == nil {
		if sys2, ok2 := plugin.api.GetSystem(systemName2); ok2 == nil {
			distance = calculateDistance(sys1.Coords, sys2.Coords)
		} else {
			logging.Trace.Println(ok2)
			err = fmt.Errorf("EDSM couldn't fetch information about %s system", systemName2)
		}
	} else {
		logging.Trace.Println(ok1)
		err = fmt.Errorf("EDSM couldn't fetch information about %s system", systemName1)
	}
	return
}

func (plugin *Plugin) getSphereSystems(systemName1 string, distance string) (systems []System, err error) {
	if value, ok1 := strconv.ParseFloat(distance, 64); ok1 == nil {
		if sysList, ok2 := plugin.api.GetSphereSystems(systemName1, value); ok2 == nil {
			systems = sysList
		} else {
			logging.Trace.Println(ok2)
			err = fmt.Errorf("EDSM couldn't fetch information about %s system", systemName1)
		}
	} else {
		logging.Trace.Println(ok1)
		err = errors.New("The distance provided is not a float number")
	}
	return
}
