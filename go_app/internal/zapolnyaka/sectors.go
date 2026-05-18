package zapolnyaka

import (
	"context"
	"fmt"
	"slices"
	"zapolnyaka/encx"
	"zapolnyaka/pkg/utils"
)

const maxAnswersPerSector = 10

// addAnswersToSector creates a new sector with the given name and adds all answers.
// Answers are split into chunks of maxAnswersPerSector; subsequent chunks are added
// via AdminUpdateSector with the full accumulated answer list.
func (z *Zapolnyaka) addAnswersToSector(ctx context.Context, level int, sectorName string, answers []string) error {
	if len(answers) == 0 {
		return nil
	}
	chunks := utils.ChunkSlice(answers, maxAnswersPerSector)

	// Create sector with the first chunk
	if err := z.client.AdminCreateSector(ctx, z.gameID, level, encx.AdminSector{
		Name:    sectorName,
		Answers: chunks[0],
	}); err != nil {
		return fmt.Errorf("create sector: %w", err)
	}

	if len(chunks) == 1 {
		return nil
	}

	// Fetch the sector ID of the newly created sector (last one with matching name)
	allSectors, err := z.client.AdminGetSectorAnswers(ctx, z.gameID, level)
	if err != nil {
		return fmt.Errorf("get sector answers: %w", err)
	}
	sectorID := 0
	for _, s := range allSectors {
		if s.Name == sectorName {
			sectorID = s.ID
		}
	}
	if sectorID == 0 {
		return fmt.Errorf("sector %q not found after creation", sectorName)
	}

	// Add remaining chunks by updating with the full accumulated answer list
	all := slices.Clone(chunks[0])
	for _, chunk := range chunks[1:] {
		all = append(all, chunk...)
		if err := z.client.AdminUpdateSector(ctx, z.gameID, level, sectorID, encx.AdminSector{
			Name:    sectorName,
			Answers: all,
		}); err != nil {
			return fmt.Errorf("update sector: %w", err)
		}
	}
	return nil
}
