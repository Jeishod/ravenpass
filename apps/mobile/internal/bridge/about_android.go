//go:build android

package bridge

// About reports Android's version and the notices packaged into the APK's assets.
type About struct{}

// System names Android and its release with the API level, such as "15 (API 35)"; empty before attach.
func (About) System() (string, string) { return "Android", string(systemVersion()) }

// Notices reads the APK's third-party notices; fs.ErrNotExist reports an APK built without them.
func (About) Notices() ([]byte, error) {
	s, text := thirdPartyNotices()
	if err := noticesError(s); err != nil {
		return nil, err
	}
	return text, nil
}
