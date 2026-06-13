package cloud

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
)

// Experience icon + thumbnails live on the legacy publishing surface (no
// cloud/v2 resource yet). PATH CHOICE (unverified against production, same
// caveat as badges.go): assumed proxied at apis.roblox.com under the
// universes/v1 prefix that already hosts place publishing — icon upload is
// POST /universes/v1/{universeId}/icon, thumbnail upload is
// POST /universes/v1/{universeId}/thumbnails, ordering is
// POST /universes/v1/{universeId}/thumbnails/order, and removal is
// DELETE /universes/v1/{universeId}/thumbnails/{thumbnailId} — mirroring
// publish.roblox.com/v1 and develop.roblox.com/v1 routes. The multipart part
// name ("file") is part of the same guess. Each URL lives in one const so a
// correction is a one-line change.
const (
	universeIconPathFmt            = "/universes/v1/%d/icon"
	universeThumbnailPathFmt       = "/universes/v1/%d/thumbnails"
	universeThumbnailOrderPathFmt  = "/universes/v1/%d/thumbnails/order"
	universeThumbnailDeletePathFmt = "/universes/v1/%d/thumbnails/%d"
)

// publishTarget is the legacy publishing response: the id of the uploaded
// icon/thumbnail image.
type publishTarget struct {
	TargetID int64 `json:"targetId"`
}

// UploadUniverseIcon uploads an experience icon image (multipart form-data,
// one "file" part) and returns the uploaded image's asset id.
func (c *Client) UploadUniverseIcon(ctx context.Context, universeID int64, fileName string, file io.Reader) (int64, error) {
	body, contentType, err := singleFileMultipart(fileName, file)
	if err != nil {
		return 0, err
	}
	var out publishTarget
	if err := c.do(ctx, "POST", fmt.Sprintf(universeIconPathFmt, universeID), nil, contentType, body, &out); err != nil {
		return 0, err
	}
	return out.TargetID, nil
}

// UploadUniverseThumbnail uploads one experience thumbnail image and returns
// its thumbnail id (used for ordering and deletion).
func (c *Client) UploadUniverseThumbnail(ctx context.Context, universeID int64, fileName string, file io.Reader) (int64, error) {
	body, contentType, err := singleFileMultipart(fileName, file)
	if err != nil {
		return 0, err
	}
	var out publishTarget
	if err := c.do(ctx, "POST", fmt.Sprintf(universeThumbnailPathFmt, universeID), nil, contentType, body, &out); err != nil {
		return 0, err
	}
	return out.TargetID, nil
}

// SetUniverseThumbnailOrder applies the display order of a universe's
// thumbnails (first id shows first).
func (c *Client) SetUniverseThumbnailOrder(ctx context.Context, universeID int64, thumbnailIDs []int64) error {
	req := struct {
		ThumbnailIDs []int64 `json:"thumbnailIds"`
	}{ThumbnailIDs: thumbnailIDs}
	return c.doJSON(ctx, "POST", fmt.Sprintf(universeThumbnailOrderPathFmt, universeID), nil, req, nil)
}

// DeleteUniverseThumbnail removes one thumbnail from the universe.
func (c *Client) DeleteUniverseThumbnail(ctx context.Context, universeID, thumbnailID int64) error {
	return c.do(ctx, "DELETE", fmt.Sprintf(universeThumbnailDeletePathFmt, universeID, thumbnailID), nil, "", nil, nil)
}

// singleFileMultipart builds a one-part multipart/form-data body carrying the
// image bytes under the "file" field, Content-Type guessed from the
// extension (shared with the assets API helper).
func singleFileMultipart(fileName string, file io.Reader) (body []byte, contentType string, err error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, fileName))
	header.Set("Content-Type", fileContentType(fileName))
	part, err := w.CreatePart(header)
	if err != nil {
		return nil, "", err
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, "", err
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), w.FormDataContentType(), nil
}
