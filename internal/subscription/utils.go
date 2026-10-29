package subscription

import "time"

type monthInDaysMap map[time.Month]int

func isLeapYear(year int) bool {
	if year%4 == 0 {
		if year%100 != 0 || year%400 == 0 {
			return true
		}
	}
	return false
}

func newMonthInDaysMap(year int) monthInDaysMap {
	monthsLenght := monthInDaysMap{
		time.Month(1):  31,
		time.Month(2):  28,
		time.Month(3):  31,
		time.Month(4):  30,
		time.Month(5):  31,
		time.Month(6):  30,
		time.Month(7):  31,
		time.Month(8):  31,
		time.Month(9):  30,
		time.Month(10): 31,
		time.Month(11): 30,
		time.Month(12): 31,
	}

	// Adjust february in case this is a leap year
	if isLeapYear(year) {
		monthsLenght[time.Month(2)] = 29
	}

	return monthsLenght
}

// daysInRange returns the sum of days for the months start to stop inclusively
func (m monthInDaysMap) daysInRange(start, stop int) int {
	var total int
	for i := start; i <= stop; i++ {
		total += m[time.Month(i)]
	}
	return total
}

// monthDifference returns the difference in months between past and present.
// It accounts for days, if the present's day has not reached the past's day the month does not count as complete.
func monthDifference(past, present time.Time) int {
	months := (present.Year()-past.Year())*12 +
		int(present.Month()-past.Month())

	// If the current day hasn't reached the past day yet,
	// the last month isn't complete.
	if present.Day() < past.Day() {
		months--
	}

	return months
}

func yearDifference(past, today time.Time) int {
	years := (today.Year() - past.Year())

	// If the current month hasn't reached the past month yet.
	// the last year isn't complete.
	if today.Month() < past.Month() {
		years--
		return years
	}

	// If the current month is the same but current day hasn't reached the past day yet.
	// the last year isn't complete.
	if today.Month() == past.Month() {
		if today.Day() < past.Day() {
			years--
		}
	}

	return years
}
