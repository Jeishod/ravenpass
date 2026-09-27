//go:build android

package bridge

import "github.com/dortanes/ravenpass/packages/app/api"

var _ api.PhotoPicker = Photos{}

// Photos picks photos with the system photo picker, which needs no storage permission.
type Photos struct{}

// Pick reads the photo the owner picks; the pick ends as canceled when the app's activity ends first.
func (Photos) Pick(limit int64) (api.PickedPhoto, bool, error) {
	s, payload := pickPhoto()
	if s == statusCanceled {
		return api.PickedPhoto{}, false, nil
	}
	if err := photoError(s); err != nil {
		return api.PickedPhoto{}, false, err
	}
	address, name, err := pickedPhoto(payload)
	if err != nil {
		return api.PickedPhoto{}, false, err
	}
	s, content := readDocument(address, limit)
	if err := photoError(s); err != nil {
		return api.PickedPhoto{}, false, err
	}
	return api.PickedPhoto{Name: name, Data: content}, true, nil
}
