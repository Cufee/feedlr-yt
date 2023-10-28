package sponsorblock

type Category struct {
	Name        string
	Description string
	Value       string
}

var (
	SelfPromo           = Category{"Self Promotion", "Unpaid or self-promotion within the video, which may include content related to merchandise, donations, or information about collaborations.", "selfpromo"}
	Interaction         = Category{"Interaction", "A brief reminder within the video to like, subscribe, or follow the content creator.", "interaction"}
	Sponsor             = Category{"Sponsor", "Paid promotions, referrals, or direct advertisements featured in the video.", "sponsor"}
	Preview             = Category{"Preview", "Clips that provide a glimpse of upcoming content or other videos in a series, with the information often repeated later in the video.", "preview"}
	Intro               = Category{"Intro Animation", "An interval featuring no actual content, such as a pause, static frame, or repeating animation.", "intro"}
	Outro               = Category{"Endcards", "The credits or appearance of YouTube endcards at the end of the video.", "outro"}
	Music               = Category{"Music", "Segments featuring off-topic music in the video.", "music_offtopic"}
	Filler              = Category{"Filler", "Tangential and humorous scenes added to the video that are not essential for understanding the main content.", "filler"}
	AvailableCategories = []Category{Sponsor, SelfPromo, Interaction, Preview, Intro, Outro, Music, Filler}
	ValidCategoryValues = []string{"sponsor", "selfpromo", "interaction", "preview", "intro", "outro", "music_offtopic", "filler"}
)
