package youtube

import "errors"

func (c *client) GetVideoByID(id string) (*Video, error) {
	details, err := c.GetVideoPlayerDetails(id)
	if err != nil {
		return nil, errors.Join(errors.New("GetVideoByID.youtube.DefaultClient.GetVideoPlayerDetails failed to get video details"), err)
	}
	return &details.Video, nil
}
