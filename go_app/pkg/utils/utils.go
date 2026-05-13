package utils

// SecondsToHMS converts total seconds to hours, minutes, seconds.
func SecondsToHMS(seconds int) (h, m, s int) {
	h = seconds / 3600
	m = (seconds % 3600) / 60
	s = seconds % 60
	return
}

// ChunkSlice splits a slice into chunks of at most size elements.
func ChunkSlice[T any](slice []T, size int) [][]T {
	if size <= 0 {
		return [][]T{slice}
	}
	var chunks [][]T
	for i := 0; i < len(slice); i += size {
		end := i + size
		if end > len(slice) {
			end = len(slice)
		}
		chunks = append(chunks, slice[i:end])
	}
	return chunks
}
