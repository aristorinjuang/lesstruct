package export

import (
	"regexp"
)

var imgSrcRe = regexp.MustCompile(`<img[^>]+src="(/uploads/media/[^"]+)"`)

func extractMediaURLs(body string) []string {
	matches := imgSrcRe.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return nil
	}
	urls := make([]string, 0, len(matches))
	for _, m := range matches {
		urls = append(urls, m[1])
	}
	return urls
}
