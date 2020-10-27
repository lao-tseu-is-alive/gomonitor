// Command screenshot is a chromedp example demonstrating how to take a
// screenshot of a specific element and of the entire browser viewport.
package main

import (
	"context"
	"flag"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/device"
	"github.com/lao-tseu-is-alive/golog"
	"io/ioutil"
	"log"
	"math"
	"time"
)

const VERSION = "0.2.0"
const BuildDate = "27-10-2020"

func main() {
	const defaultUrl = "https://carto.lausanne.ch/"
	const defaultFileName = "screenshot.jpg"
	golog.Info("## STARTING GOMONITOR VERSION %s, BUILD AT %s", VERSION, BuildDate)
	url2Monitor := flag.String("url", defaultUrl, "url you want to get screenshot")
	filename := flag.String("filename", defaultFileName, "filename to save screenshot")
	flag.Parse()
	golog.Info("## will try url: %s, and save screenshot here: %s", *url2Monitor, *filename)
	// create context
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()
	// capture screenshot of an element
	var buf []byte

	// capture entire browser viewport, returning png with quality=90
	if err := chromedp.Run(ctx, fullScreenshot(*url2Monitor, 90, &buf)); err != nil {
		log.Fatal(err)
	}
	if err := ioutil.WriteFile(*filename, buf, 0644); err != nil {
		log.Fatal(err)
	}
}

// fullScreenshot takes a screenshot of the entire browser viewport.
func fullScreenshot(url string, quality int64, res *[]byte) chromedp.Tasks {
	// The first task enables lifecycle events on the page and the second task navigates,
	// and blocks until the desired event is received
	return chromedp.Tasks{
		enableLifeCycleEvents(),
		chromedp.Emulate(device.IPad),
		chromedp.EmulateViewport(1024, 768, chromedp.EmulateScale(2)),
		// chromedp.Navigate(url),
		navigateAndWaitFor(url, "networkIdle"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			// get layout metrics
			_, _, contentSize, err := page.GetLayoutMetrics().Do(ctx)
			if err != nil {
				golog.Err("ERROR getting page.GetLayoutMetrics : %v", err)
				return err
			}

			width, height := int64(math.Ceil(contentSize.Width)), int64(math.Ceil(contentSize.Height))

			// force viewport emulation
			err = emulation.SetDeviceMetricsOverride(int64(width), int64(height), 1, false).
				WithScreenOrientation(&emulation.ScreenOrientation{
					Type:  emulation.OrientationTypeLandscapePrimary,
					Angle: 0,
				}).
				Do(ctx)
			if err != nil {
				golog.Err("ERROR doing  emulation.SetDeviceMetricsOverride : %v", err)
				return err
			}

			// capture screenshot
			*res, err = page.CaptureScreenshot().
				WithFormat(page.CaptureScreenshotFormatJpeg).
				WithQuality(quality).
				WithClip(&page.Viewport{
					X:      contentSize.X,
					Y:      contentSize.Y,
					Width:  contentSize.Width,
					Height: contentSize.Height,
					Scale:  1,
				}).Do(ctx)
			if err != nil {
				golog.Err("ERROR doing  page.CaptureScreenshot : %v", err)
				return err
			}
			return nil
		}),
	}
}

// https://github.com/chromedp/chromedp/issues/431
func enableLifeCycleEvents() chromedp.ActionFunc {
	return func(ctx context.Context) error {
		err := page.Enable().Do(ctx)
		if err != nil {
			return err
		}
		err = page.SetLifecycleEventsEnabled(true).Do(ctx)
		if err != nil {
			return err
		}
		return nil
	}
}

func navigateAndWaitFor(url string, eventName string) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		_, _, _, err := page.Navigate(url).Do(ctx)
		if err != nil {
			golog.Err("Error doing page.Navigate : %v", err)
			return err
		}

		return waitFor(ctx, eventName)
		return nil
	}
}

// waitFor blocks until eventName is received.
// Examples of events you can wait for:
//     init, DOMContentLoaded, firstPaint,
//     firstContentfulPaint, firstImagePaint,
//     firstMeaningfulPaintCandidate,
//     load, networkAlmostIdle, firstMeaningfulPaint, networkIdle
//
// This is not super reliable, I've already found incidental cases where
// networkIdle was sent before load. It's probably smart to see how
// puppeteer implements this exactly.
func waitFor(ctx context.Context, eventName string) error {
	ch := make(chan struct{})
	cctx, cancel := context.WithTimeout(ctx, time.Duration(60)*time.Second) // let's have a 1m timeout
	chromedp.ListenTarget(cctx, func(ev interface{}) {
		switch e := ev.(type) {
		case *page.EventLifecycleEvent:
			if e.Name == eventName {
				cancel()
				close(ch)
			}
		}
	})
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}

}
