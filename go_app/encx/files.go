package encx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

// AdminUploadGameFile uploads a single file to the game's "Файлы для игры"
// storage via FileUploader.aspx (multipart/form-data). After upload the file is
// served from https://d1.endata.cx/data/games/<gameId>/<filename> (instant, no
// cache — handy for development) and https://cdn.endata.cx/data/games/<gameId>/<filename>
// (24h cache — production).
//
// The form has no __VIEWSTATE: it carries only the file field (inputFile1) and
// an <input type="image" name="ctl03"> submit, so we also send the click
// coordinates ctl03.x / ctl03.y. The game is identified solely by the gid query
// string. The server limits each file to 48 MB.
func (c *Client) AdminUploadGameFile(ctx context.Context, gameId int, filename string, content io.Reader) error {
	u := fmt.Sprintf("%s/Administration/Games/FileUploader.aspx?gid=%d", c.baseURL(), gameId)

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)

	part, err := mw.CreateFormFile("inputFile1", filename)
	if err != nil {
		return fmt.Errorf("encx: create file part: %w", err)
	}
	if _, err := io.Copy(part, content); err != nil {
		return fmt.Errorf("encx: write file part: %w", err)
	}

	// Emulate a click on the image submit button.
	_ = mw.WriteField("ctl03.x", "1")
	_ = mw.WriteField("ctl03.y", "1")

	if err := mw.Close(); err != nil {
		return fmt.Errorf("encx: close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, &body)
	if err != nil {
		return fmt.Errorf("encx: create upload request: %w", err)
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("encx: upload %s: %w", filename, err)
	}
	defer resp.Body.Close()

	if isLoginRedirect(resp) {
		return fmt.Errorf("encx: upload %s: %w", filename, ErrSessionExpired)
	}

	// Drain the body so the connection can be reused.
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("encx: upload %s: HTTP %d", filename, resp.StatusCode)
	}
	return nil
}
