package youtube

func (c *client) GetVideoDetailsByID(id string) (*VideoDetails, error) {
	details, err := c.GetVideoPlayerDetails(id)
	if err != nil {
		return nil, err
	}
	return details, nil
}
