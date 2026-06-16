package seo

import (
	"fmt"
	"time"
)

const (
	changeFrequencyDaily  = "daily"
	changeFrequencyWeekly = "weekly"
	priorityHomepage      = "1.0"
	priorityPost          = "0.8"
	priorityPage          = "0.6"
	postTypePost          = "post"
)

type ContentItem struct {
	Slug      string
	UpdatedAt string
}

type SitemapEntry struct {
	Loc        string
	LastMod    string
	ChangeFreq string
	Priority   string
}

func ContentToSitemapEntry(baseURL string, content ContentItem, postType string) SitemapEntry {
	var loc string
	if postType == postTypePost {
		loc = fmt.Sprintf("%s/posts/%s", baseURL, content.Slug)
	} else {
		loc = fmt.Sprintf("%s/%s", baseURL, content.Slug)
	}

	return SitemapEntry{
		Loc:        loc,
		LastMod:    content.UpdatedAt,
		ChangeFreq: GetChangeFrequency(postType),
		Priority:   GetPriority(postType),
	}
}

func GetChangeFrequency(postType string) string {
	switch postType {
	case postTypePost:
		return changeFrequencyDaily
	default:
		return changeFrequencyWeekly
	}
}

func GetPriority(postType string) string {
	switch postType {
	case postTypePost:
		return priorityPost
	default:
		return priorityPage
	}
}

func NewHomepageEntry(baseURL string) SitemapEntry {
	return SitemapEntry{
		Loc:        fmt.Sprintf("%s/", baseURL),
		LastMod:    time.Now().Format(time.RFC3339),
		ChangeFreq: changeFrequencyDaily,
		Priority:   priorityHomepage,
	}
}
