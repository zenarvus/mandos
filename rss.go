package main

import (
	"time"
	"strings"
	"encoding/xml"
)

//To show the time aware tasks in the rss feed
type sortNodesByDate []servedFile
func (a sortNodesByDate) Len() int { return len(a) }
func (a sortNodesByDate) Less(i, j int) bool {
	// newest/biggest first
	return getTimeFromStr(a[j].Metadata["date"]).Unix() < getTimeFromStr(a[i].Metadata["date"]).Unix()
}
func (a sortNodesByDate) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

func getTimeFromStr(timeStr string) (time.Time) {
	timeStr = strings.ReplaceAll(timeStr, "-", "")
	timeStr = strings.ReplaceAll(timeStr, "/", "")
	formats := []string{"20060102","02012006",}

	var parsedTime time.Time
	var err error
	for _, format := range formats {
		parsedTime, err = time.Parse(format, timeStr)
		if err == nil {
			return parsedTime // Return if parsing succeeds
		}
	}
	return time.Time{}
}

type rssFeed struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Channel channel  `xml:"channel"`
}
type channel struct {
	Title	   string `xml:"title"`
	Link		string `xml:"link"`
	Description string `xml:"description"`
	PubDate	 string `xml:"pubDate"`
	Items	   []rssItem `xml:"item"`
}
type rssItem struct {
	Title	   string `xml:"title"`
	Link		string `xml:"link"`
	Description string `xml:"description"`
	PubDate	 string `xml:"pubDate"`
	/*Enclosure   enclosure  `xml:"enclosure"`*/
}
type enclosure struct {
	URL	string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Length int64  `xml:"length,attr"`
}
func ConvertToRSS(nodes []servedFile, baseUrl, title string) (rssFeed) {
	rss := rssFeed{
		Version: "2.0",
		Channel: channel{
			Title: title,
			Link: baseUrl+"/rss",
			//Description: description,
			PubDate: time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT"),
		},
	}
	rssItems := make([]rssItem, 0)
	for _, node := range nodes {
		rssItem := rssItem{
			Title:	   node.Title,
			Link:		 baseUrl+node.MapKey,
			//Description: node.Content,
			PubDate:	 getTimeFromStr(node.Metadata["date"]).Format("Mon, 02 Jan 2006 15:04:05 GMT"),
			/*Enclosure: Enclosure{
				URL:	BaseURL + "/public/uploads/post_image/" + post.Image, // Provide the image URL for each post
				Type:   "image/jpeg", // Specify the appropriate MIME type for the image
				Length: 0,   
			},*/
		}

		// Check if the post already exists in rssItems
		found := false
		for i, existingItem := range rssItems {
			if existingItem.Link == rssItem.Link {
				// Update the existing item with the modified values
				rssItems[i] = rssItem
				found = true
				break
			}
		}
		// Add the new item to rssItems
		if !found {rssItems = append(rssItems, rssItem)}
	}
	rss.Channel.Items = rssItems
	
	return rss
}
