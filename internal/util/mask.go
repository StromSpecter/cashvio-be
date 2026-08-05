package util

import "fmt"

func MaskCardNumber(number string) string {
	if len(number) < 4 {
		return "•••• ••••"
	}
	return fmt.Sprintf("•••• •••• %s", number[len(number)-4:])
}

func MaskWalletNumber(number string) string {
	if len(number) < 4 {
		return "•••• ••••"
	}
	return fmt.Sprintf("•••• %s", number[len(number)-4:])
}
