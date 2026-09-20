package service

import (
	"fmt"
)

var (
	planets = []string{
		"Mercury",
		"Venus",
		"Earth",
		"Mars",
		"Jupiter",
		"Saturn",
		"Uranus",
		"Neptune",
	}

	qualities = []string{
		"Serenity",
		"Elegance",
		"Grandeur",
		"Harmony",
		"Splendour",
		"Tranquillity",
		"Radiance",
		"Majesty",
	}
)

func hotelName(index int) string {
	planet := planets[(index-1)%len(planets)]
	quality := qualities[((index-1)/len(planets))%len(qualities)]

	return fmt.Sprintf("%d %s %s", index, planet, quality)
}
