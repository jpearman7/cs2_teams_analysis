package main

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs"
	"github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs/common"
	"github.com/markus-wa/demoinfocs-golang/v4/pkg/demoinfocs/events"
)

type DamageInfo struct {
	Attacker          string
	Target            string
	HealthDamageTaken int
	ArmorDamageTaken  int
}

type PlayerData struct {
	Name              string
	IsBot             bool
	PlayerID          int
	Team              common.Team
	TeamID            int
	TeamScore         int
	RoundNumber       int
	Tick              int
	Time              float64
	ClockTime         float64
	PosX              float32
	PosY              float32
	PosZ              float32
	VelX              float32
	VelY              float32
	VelZ              float32
	ViewDirectionX    float32
	ViewDirectionY    float32
	KillEvent         bool
	KilledBy          string
	Killed            string
	Assisters         string
	Kills             int
	Deaths            int
	Assists           int
	Attacked          bool
	AttackedBy        []string
	Attacking         bool
	AttackingTarget   string
	Health            int
	HealthDamageTaken int
	Armor             int
	ArmorDamageTaken  int
	BombPlantBegin    bool
	BombPlanted       bool
	BombDefuseStarted bool
	BombDefused       bool
	BombPlantSite     string
	BombDefuseSite    string
	FlashDuration     float32
}

func main() {
	// Specify the directory containing .dem files
	demoDir := "C:/Users/jjp77/OneDrive/LINC Lab/E-Sports/CS2_dems"

	// Create Outputs directory if it doesn't exist
	outputDir := "Outputs"
	if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
		fmt.Println("Error creating Outputs directory:", err)
		return
	}

	// Scan for .dem files
	demoFiles, err := filepath.Glob(filepath.Join(demoDir, "*.dem"))
	if err != nil {
		fmt.Println("Error scanning for .dem files:", err)
		return
	}

	// Process each .dem file
	for _, demoFile := range demoFiles {
		err := processDemo(demoFile, outputDir)
		if err != nil {
			fmt.Printf("Error processing %s: %v\n", demoFile, err)
		}
	}
}

func processDemo(demoFile, outputDir string) error {
	fmt.Printf("Processing file: %s\n", demoFile)

	// Defer a function to catch and log panics
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Recovered from panic in processDemo: %v\n", r)
			debug.PrintStack()
		}
	}()

	file, err := os.Open(demoFile)
	if err != nil {
		return fmt.Errorf("error opening file %s: %v", demoFile, err)
	}
	defer file.Close()

	parser := demoinfocs.NewParser(file)
	defer parser.Close()

	_, err = parser.ParseHeader()
	if err != nil {
		return fmt.Errorf("error parsing header of %s: %v", demoFile, err)
	}

	outputFile := filepath.Join(outputDir, strings.TrimSuffix(filepath.Base(demoFile), ".dem")+".csv")
	output, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("error creating output file for %s: %v", demoFile, err)
	}
	defer output.Close()

	writer := csv.NewWriter(output)
	defer writer.Flush()

	// Write CSV header
	writer.Write([]string{
		"Name", "IsBot", "PlayerID", "Team", "TeamID", "TeamScore", "RoundNumber", "Tick", "Time", "ClockTime",
		"PosX", "PosY", "PosZ",
		"VelX", "VelY", "VelZ",
		"ViewDirectionX", "ViewDirectionY",
		"KillEvent", "KilledBy", "Killed", "Assisters", "Kills", "Deaths", "Assists",
		"Attacked", "AttackedBy", "Attacking", "AttackingTarget", "Health", "HealthDamageTaken", "Armor", "ArmorDamageTaken",
		"BombPlantBegin", "BombPlanted", "BombDefuseStarted", "BombDefused",
		"BombPlantSite", "BombDefuseSite", "FlashDuration",
	})

	var roundNumber int
	var roundStartTime float64
	const roundDuration float64 = 115 // 1:55 in seconds

	var lastProcessedSecond int = -1
	killEventData := make(map[string][]*events.Kill)
	damageInfo := make(map[string]map[string]*DamageInfo)
	var bombPlantedData *events.BombPlanted
	var bombDefusedData *events.BombDefused
	assistInfo := make(map[string][]string)
	bombPlantBeginData := make(map[string]bool)
	bombDefuseStartData := make(map[string]bool)

	// Round start handler
	parser.RegisterEventHandler(func(e events.RoundStart) {
		roundNumber++
		roundStartTime = float64(parser.GameState().IngameTick()) / parser.TickRate()
		// Clear state at round start
		bombPlantBeginData = make(map[string]bool)
		bombDefuseStartData = make(map[string]bool)
		bombPlantedData = nil
		bombDefusedData = nil
	})

	// Kill event handler
	parser.RegisterEventHandler(func(e events.Kill) {
		if e.Killer != nil {
			// Append kill event to killer's list
			killEventData[e.Killer.Name] = append(killEventData[e.Killer.Name], &e)
		}
		if e.Victim != nil {
			// Append kill event to victim's list
			killEventData[e.Victim.Name] = append(killEventData[e.Victim.Name], &e)
		}
		if e.Assister != nil {
			// Append assist to assister's list
			if e.Victim != nil {
				assistInfo[e.Assister.Name] = append(assistInfo[e.Assister.Name], e.Victim.Name)
			}
		}
	})

	// Damage event handler
	parser.RegisterEventHandler(func(e events.PlayerHurt) {
		if e.Player == nil || e.Attacker == nil {
			return
		}

		victim := e.Player
		attacker := e.Attacker

		if _, exists := damageInfo[victim.Name]; !exists {
			damageInfo[victim.Name] = make(map[string]*DamageInfo)
		}
		if _, exists := damageInfo[attacker.Name]; !exists {
			damageInfo[attacker.Name] = make(map[string]*DamageInfo)
		}

		attackerName := attacker.Name
		if _, exists := damageInfo[victim.Name][attackerName]; !exists {
			damageInfo[victim.Name][attackerName] = &DamageInfo{
				Attacker: attackerName,
				Target:   victim.Name,
			}
		}

		// Record damage on attacker's side
		victimName := victim.Name
		if _, exists := damageInfo[attacker.Name][victimName]; !exists {
			damageInfo[attacker.Name][victimName] = &DamageInfo{
				Attacker: attackerName,
				Target:   victimName,
			}
		}

		// Update damage values
		damageInfo[victim.Name][attackerName].HealthDamageTaken += e.HealthDamageTaken
		damageInfo[victim.Name][attackerName].ArmorDamageTaken += e.ArmorDamageTaken
		damageInfo[attacker.Name][victimName].HealthDamageTaken += e.HealthDamageTaken
		damageInfo[attacker.Name][victimName].ArmorDamageTaken += e.ArmorDamageTaken
	})

	// Bomb event handlers
	parser.RegisterEventHandler(func(e events.BombPlanted) {
		bombPlantedData = &e
	})

	parser.RegisterEventHandler(func(e events.BombDefused) {
		bombDefusedData = &e
	})

	parser.RegisterEventHandler(func(e events.BombPlantBegin) {
		if e.Player != nil {
			bombPlantBeginData[e.Player.Name] = true
		}
	})

	parser.RegisterEventHandler(func(e events.BombPlantAborted) {
		if e.Player != nil {
			bombPlantBeginData[e.Player.Name] = false
		}
	})

	parser.RegisterEventHandler(func(e events.BombDefuseStart) {
		if e.Player != nil {
			bombDefuseStartData[e.Player.Name] = true
		}
	})

	parser.RegisterEventHandler(func(e events.BombDefuseAborted) {
		if e.Player != nil {
			bombDefuseStartData[e.Player.Name] = false
		}
	})

	// Frame processing
	parser.RegisterEventHandler(func(e events.FrameDone) {
		tick := parser.GameState().IngameTick()
		gameTime := float64(tick) / parser.TickRate()
		currentSecond := int(gameTime)

		if currentSecond > lastProcessedSecond {
			clockTime := roundDuration - (gameTime - roundStartTime)
			if clockTime < 0 {
				clockTime = 0
			}

			for _, player := range parser.GameState().Participants().Playing() {
				if player == nil {
					continue
				}

				teamState := player.TeamState
				if teamState == nil {
					continue
				}

				playerData := PlayerData{
					Name:              player.Name,
					IsBot:             player.IsBot,
					PlayerID:          player.UserID,
					Team:              player.Team,
					TeamID:            teamState.ID(),
					TeamScore:         teamState.Score(),
					RoundNumber:       roundNumber,
					Tick:              tick,
					Time:              gameTime,
					ClockTime:         math.Round(clockTime*100) / 100,
					PosX:              float32(player.Position().X),
					PosY:              float32(player.Position().Y),
					PosZ:              float32(player.Position().Z),
					KillEvent:         false,
					KilledBy:          "",
					Killed:            "",
					Assisters:         "",
					Attacked:          false,
					AttackedBy:        []string{},
					Health:            player.Health(),
					HealthDamageTaken: 0,
					Armor:             player.Armor(),
					ArmorDamageTaken:  0,
					BombPlantBegin:    bombPlantBeginData[player.Name],
					BombPlanted:       false,
					BombDefuseStarted: bombDefuseStartData[player.Name],
					BombDefused:       false,
					BombPlantSite:     "",
					BombDefuseSite:    "",
					FlashDuration:     player.FlashDuration,
				}

				// Calculate velocity
				vel := player.Velocity()
				playerData.VelX = float32(vel.X)
				playerData.VelY = float32(vel.Y)
				playerData.VelZ = float32(vel.Z)

				// Get view direction
				playerData.ViewDirectionX = float32(player.ViewDirectionX())
				playerData.ViewDirectionY = float32(player.ViewDirectionY())

				// Get player stats
				playerData.Kills = player.Kills()
				playerData.Deaths = player.Deaths()
				playerData.Assists = player.Assists()

				// Process kill events
				if killEvents, ok := killEventData[player.Name]; ok {
					for _, killEvent := range killEvents {
						if killEvent.Killer != nil && killEvent.Killer.Name == player.Name {
							playerData.KillEvent = true
							if killEvent.Victim != nil {
								if playerData.Killed != "" {
									playerData.Killed += "," + killEvent.Victim.Name
								} else {
									playerData.Killed = killEvent.Victim.Name
								}
							}
						}
						if killEvent.Victim != nil && killEvent.Victim.Name == player.Name {
							playerData.KillEvent = true
							if killEvent.Killer != nil {
								if playerData.KilledBy != "" {
									playerData.KilledBy += "," + killEvent.Killer.Name
								} else {
									playerData.KilledBy = killEvent.Killer.Name
								}
							}
						}
					}
				}

				// Process assists
				if assists, ok := assistInfo[player.Name]; ok && len(assists) > 0 {
					playerData.Assisters = strings.Join(assists, ",")
				}

				// Process damage info
				if playerDamageInfo, exists := damageInfo[player.Name]; exists {
					var attackingTargets []string
					for _, info := range playerDamageInfo {
						if info.Attacker == player.Name {
							// This player is the attacker
							playerData.Attacking = true
							attackingTargets = append(attackingTargets, info.Target)
						}
						if info.Target == player.Name {
							// This player is being attacked
							playerData.Attacked = true
							playerData.AttackedBy = append(playerData.AttackedBy, info.Attacker)
							playerData.HealthDamageTaken += info.HealthDamageTaken
							playerData.ArmorDamageTaken += info.ArmorDamageTaken
						}
					}
					if len(attackingTargets) > 0 {
						playerData.AttackingTarget = strings.Join(attackingTargets, ",")
					}
				}

				// Process bomb events
				if bombPlantedData != nil && bombPlantedData.Player == player {
					playerData.BombPlanted = true
					playerData.BombPlantSite = string(bombPlantedData.Site)
				}
				if bombDefusedData != nil && bombDefusedData.Player == player {
					playerData.BombDefused = true
					playerData.BombDefuseSite = string(bombDefusedData.Site)
				}

				record := []string{
					playerData.Name,
					strconv.FormatBool(playerData.IsBot),
					strconv.Itoa(playerData.PlayerID),
					teamToString(playerData.Team),
					strconv.Itoa(playerData.TeamID),
					strconv.Itoa(playerData.TeamScore),
					strconv.Itoa(playerData.RoundNumber),
					strconv.Itoa(playerData.Tick),
					strconv.FormatFloat(playerData.Time, 'f', 2, 64),
					strconv.FormatFloat(playerData.ClockTime, 'f', 2, 64),
					strconv.FormatFloat(float64(playerData.PosX), 'f', 6, 32),
					strconv.FormatFloat(float64(playerData.PosY), 'f', 6, 32),
					strconv.FormatFloat(float64(playerData.PosZ), 'f', 6, 32),
					strconv.FormatFloat(float64(playerData.VelX), 'f', 6, 32),
					strconv.FormatFloat(float64(playerData.VelY), 'f', 6, 32),
					strconv.FormatFloat(float64(playerData.VelZ), 'f', 6, 32),
					strconv.FormatFloat(float64(playerData.ViewDirectionX), 'f', 6, 32),
					strconv.FormatFloat(float64(playerData.ViewDirectionY), 'f', 6, 32),
					strconv.FormatBool(playerData.KillEvent),
					playerData.KilledBy,
					playerData.Killed,
					playerData.Assisters,
					strconv.Itoa(playerData.Kills),
					strconv.Itoa(playerData.Deaths),
					strconv.Itoa(playerData.Assists),
					strconv.FormatBool(playerData.Attacked),
					strings.Join(playerData.AttackedBy, ","),
					strconv.FormatBool(playerData.Attacking),
					playerData.AttackingTarget,
					strconv.Itoa(playerData.Health),
					strconv.Itoa(playerData.HealthDamageTaken),
					strconv.Itoa(playerData.Armor),
					strconv.Itoa(playerData.ArmorDamageTaken),
					strconv.FormatBool(playerData.BombPlantBegin),
					strconv.FormatBool(playerData.BombPlanted),
					strconv.FormatBool(playerData.BombDefuseStarted),
					strconv.FormatBool(playerData.BombDefused),
					playerData.BombPlantSite,
					playerData.BombDefuseSite,
					strconv.FormatFloat(float64(playerData.FlashDuration), 'f', 2, 32),
				}

				if err := writer.Write(record); err != nil {
					fmt.Printf("Error writing record to csv for %s: %v\n", demoFile, err)
				}
			}
			killEventData = make(map[string][]*events.Kill)
			damageInfo = make(map[string]map[string]*DamageInfo)
			assistInfo = make(map[string][]string)
			lastProcessedSecond = currentSecond
		}
	})

	err = parser.ParseToEnd()
	if err != nil {
		return fmt.Errorf("error parsing demo %s: %v", demoFile, err)
	}

	fmt.Printf("Finished processing file: %s\n", demoFile)
	return nil
}

// Helper function to convert the common.Team type to a string
func teamToString(team common.Team) string {
	switch team {
	case common.TeamUnassigned:
		return "Unassigned"
	case common.TeamSpectators:
		return "Spectators"
	case common.TeamTerrorists:
		return "Terrorists"
	case common.TeamCounterTerrorists:
		return "CounterTerrorists"
	default:
		return "Unknown"
	}
}
