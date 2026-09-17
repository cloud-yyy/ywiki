package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

// uploadPartSize is the chunk size used for multipart uploads. The API accepts
// parts between 5 MB and 16 MB, except for the final part.
const uploadPartSize = 8 * 1024 * 1024

// ListAttachmentsOptions tunes an attachment listing.
type ListAttachmentsOptions struct {
	// PageSize is the number of results per page.
	PageSize int

	// Cursor continues a previous listing.
	Cursor string
}

// ListAttachments lists the files attached to a page.
func (c *Client) ListAttachments(
	ctx context.Context, pageID int, opts ListAttachmentsOptions,
) (*ListResult[Attachment], error) {
	query := url.Values{}
	if opts.PageSize > 0 {
		query.Set("page_size", strconv.Itoa(opts.PageSize))
	}
	if opts.Cursor != "" {
		query.Set("cursor", opts.Cursor)
	}

	var list cursorList[Attachment]
	if err := c.do(ctx, request{
		method: http.MethodGet,
		path:   "/pages/" + strconv.Itoa(pageID) + "/attachments",
		query:  query,
	}, &list); err != nil {
		return nil, err
	}

	return &ListResult[Attachment]{Items: list.Results, NextCursor: list.NextCursor}, nil
}

// DownloadAttachment opens an attachment for reading by file ID.
// The caller must close the returned reader.
func (c *Client) DownloadAttachment(
	ctx context.Context, pageID, fileID int,
) (io.ReadCloser, error) {
	return c.doRaw(ctx, request{
		method: http.MethodGet,
		path: "/pages/" + strconv.Itoa(pageID) +
			"/attachments/" + strconv.Itoa(fileID) + "/download",
	})
}

// DeleteAttachment removes an attached file from a page.
func (c *Client) DeleteAttachment(ctx context.Context, pageID, fileID int) error {
	return c.do(ctx, request{
		method: http.MethodDelete,
		path: "/pages/" + strconv.Itoa(pageID) +
			"/attachments/" + strconv.Itoa(fileID),
	}, nil)
}

// CreateUploadSession opens a multipart upload session for a file.
func (c *Client) CreateUploadSession(
	ctx context.Context, fileName string, fileSize int64,
) (*UploadSession, error) {
	var session UploadSession
	if err := c.do(ctx, request{
		method: http.MethodPost,
		path:   "/upload_sessions",
		body: struct {
			FileName string `json:"file_name"`
			FileSize int64  `json:"file_size"`
		}{FileName: fileName, FileSize: fileSize},
	}, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

// UploadPart uploads one chunk of a file. Part numbers start at 1.
func (c *Client) UploadPart(
	ctx context.Context, sessionID string, partNumber int, body io.Reader,
) error {
	query := url.Values{}
	query.Set("part_number", strconv.Itoa(partNumber))

	return c.do(ctx, request{
		method:  http.MethodPut,
		path:    "/upload_sessions/" + url.PathEscape(sessionID) + "/upload_part",
		query:   query,
		rawBody: body,
	}, nil)
}

// FinishUploadSession closes an upload session so the file can be attached.
func (c *Client) FinishUploadSession(ctx context.Context, sessionID string) (*UploadSession, error) {
	var session UploadSession
	if err := c.do(ctx, request{
		method: http.MethodPost,
		path:   "/upload_sessions/" + url.PathEscape(sessionID) + "/finish",
	}, &session); err != nil {
		return nil, err
	}

	return &session, nil
}

// AttachUploadedFiles attaches files from completed upload sessions to a page.
func (c *Client) AttachUploadedFiles(
	ctx context.Context, pageID int, sessionIDs []string,
) ([]Attachment, error) {
	var list cursorList[Attachment]
	if err := c.do(ctx, request{
		method: http.MethodPost,
		path:   "/pages/" + strconv.Itoa(pageID) + "/attachments",
		body: struct {
			UploadSessions []string `json:"upload_sessions"`
		}{UploadSessions: sessionIDs},
	}, &list); err != nil {
		return nil, err
	}

	return list.Results, nil
}

// UploadAttachment runs the full upload flow for one file: open a session,
// send the file in parts, finish the session, and attach it to the page.
func (c *Client) UploadAttachment(
	ctx context.Context, pageID int, fileName string, fileSize int64, file io.Reader,
) ([]Attachment, error) {
	session, err := c.CreateUploadSession(ctx, fileName, fileSize)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, uploadPartSize)
	for part := 1; ; part++ {
		n, readErr := io.ReadFull(file, buf)
		if n > 0 {
			if err := c.UploadPart(ctx, session.SessionID, part, bytes.NewReader(buf[:n])); err != nil {
				return nil, err
			}
		}
		if errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("failed to read %s: %w", fileName, readErr)
		}
	}

	if _, err := c.FinishUploadSession(ctx, session.SessionID); err != nil {
		return nil, err
	}

	return c.AttachUploadedFiles(ctx, pageID, []string{session.SessionID})
}
