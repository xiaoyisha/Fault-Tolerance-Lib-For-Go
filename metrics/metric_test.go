package metrics

import (
	"Perseus/config"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestErrorPercent(t *testing.T) {
	Convey("with a metric failing 40 percent of the time", t, func() {
		m := MetricFailingPercent(40)
		now := time.Now()

		Convey("ErrorPercent() should return 40", func() {
			p := m.ErrorPercent(now)
			So(p, ShouldEqual, 40)
		})

		Convey("and a error threshold set to 39", func() {
			config.ConfigureCommand("", config.CommandConfig{ErrorPercentThreshold: 39})

			Convey("the metrics should be unhealthy", func() {
				So(m.IsHealthy(now), ShouldBeFalse)
			})

		})
	})
}
