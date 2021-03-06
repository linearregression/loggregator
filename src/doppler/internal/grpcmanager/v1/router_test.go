package v1_test

import (
	"doppler/internal/grpcmanager/v1"
	"plumbing"

	. "github.com/apoydence/eachers"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Router", func() {
	var (
		// Streams without type filter
		mockDataSetterA *mockDataSetter
		mockDataSetterB *mockDataSetter
		mockDataSetterC *mockDataSetter

		// Firehoses
		mockDataSetterD *mockDataSetter
		mockDataSetterE *mockDataSetter
		mockDataSetterF *mockDataSetter

		// Streams with type filter
		mockDataSetterG *mockDataSetter

		counterEnvelope      *events.Envelope
		logEnvelope          *events.Envelope
		counterEnvelopeBytes []byte
		logEnvelopeBytes     []byte

		router *v1.Router
	)

	BeforeEach(func() {
		mockDataSetterA = newMockDataSetter()
		mockDataSetterB = newMockDataSetter()
		mockDataSetterC = newMockDataSetter()
		mockDataSetterD = newMockDataSetter()
		mockDataSetterE = newMockDataSetter()
		mockDataSetterF = newMockDataSetter()
		mockDataSetterG = newMockDataSetter()

		counterEnvelope = &events.Envelope{
			Origin:    proto.String("some-origin"),
			EventType: events.Envelope_CounterEvent.Enum(),
		}

		logEnvelope = &events.Envelope{
			Origin:    proto.String("some-origin"),
			EventType: events.Envelope_LogMessage.Enum(),
		}
		var err error
		counterEnvelopeBytes, err = counterEnvelope.Marshal()
		Expect(err).ToNot(HaveOccurred())

		logEnvelopeBytes, err = logEnvelope.Marshal()
		Expect(err).ToNot(HaveOccurred())

		router = v1.NewRouter()
	})

	Describe("data routing", func() {
		Context("when multiple setters are routed", func() {
			var (
				reqA, reqB, reqC, reqD, reqE, reqF, reqG                             *plumbing.SubscriptionRequest
				cleanupA, cleanupB, cleanupC, cleanupD, cleanupE, cleanupF, cleanupG func()
			)

			BeforeEach(func() {
				reqA = &plumbing.SubscriptionRequest{
					Filter: &plumbing.Filter{
						AppID: "some-app-id",
					},
				}

				reqB = &plumbing.SubscriptionRequest{
					Filter: &plumbing.Filter{
						AppID: "some-app-id",
					},
				}

				reqC = &plumbing.SubscriptionRequest{
					Filter: &plumbing.Filter{
						AppID: "some-other-app-id",
					},
				}

				reqD = &plumbing.SubscriptionRequest{
					ShardID: "some-sub-id",
				}

				reqE = &plumbing.SubscriptionRequest{
					ShardID: "some-sub-id",
				}

				reqF = &plumbing.SubscriptionRequest{
					ShardID: "some-other-sub-id",
				}

				reqG = &plumbing.SubscriptionRequest{
					Filter: &plumbing.Filter{
						AppID: "some-app-id",
						Message: &plumbing.Filter_Log{
							Log: &plumbing.LogFilter{},
						},
					},
				}

				// Streams without type filters
				cleanupA = router.Register(reqA, mockDataSetterA)
				cleanupB = router.Register(reqB, mockDataSetterB)
				cleanupC = router.Register(reqC, mockDataSetterC)

				// Firehose
				cleanupD = router.Register(reqD, mockDataSetterD)
				cleanupE = router.Register(reqE, mockDataSetterE)
				cleanupF = router.Register(reqF, mockDataSetterF)

				// Streams with type filters
				cleanupG = router.Register(reqG, mockDataSetterG)
			})

			It("sends data to the registered setters", func() {
				router.SendTo("some-app-id", logEnvelope)

				Expect(mockDataSetterA.SetInput).To(
					BeCalled(With(logEnvelopeBytes)),
				)

				Expect(mockDataSetterB.SetInput).To(
					BeCalled(With(logEnvelopeBytes)),
				)

				Expect(mockDataSetterG.SetInput).To(
					BeCalled(With(logEnvelopeBytes)),
				)
			})

			It("only sends the envelope to a subscription once", func() {
				router.SendTo("some-app-id", counterEnvelope)

				Expect(mockDataSetterA.SetCalled).To(
					HaveLen(1),
				)
			})

			It("does not send data to the wrong setter", func() {
				router.SendTo("some-app-id", counterEnvelope)

				Expect(mockDataSetterC.SetCalled).To(
					Not(BeCalled()),
				)

				Expect(mockDataSetterG.SetCalled).To(
					Not(BeCalled()),
				)
			})

			It("sends to a random firehose subscription", func() {
				router.SendTo("some-app-id", counterEnvelope)
				combinedLen := len(mockDataSetterD.SetCalled) + len(mockDataSetterE.SetCalled)

				Expect(combinedLen).To(Equal(1))
				Expect(mockDataSetterF.SetInput).To(
					BeCalled(With(counterEnvelopeBytes)),
				)
			})

			It("does not send data for bad envelope", func() {
				router.SendTo("some-app-id", new(events.Envelope))

				Expect(mockDataSetterA.SetCalled).To(
					Not(BeCalled()),
				)

				Expect(mockDataSetterD.SetCalled).To(
					Not(BeCalled()),
				)

				Expect(mockDataSetterE.SetCalled).To(
					Not(BeCalled()),
				)
			})

			Context("when one stream setter is unregistered", func() {
				BeforeEach(func() {
					cleanupA()
				})

				It("does not send data to that setter", func() {
					router.SendTo("some-app-id", counterEnvelope)

					Expect(mockDataSetterA.SetCalled).To(
						Not(BeCalled()),
					)
				})

				It("does send data to the remaining registered setter", func() {
					router.SendTo("some-app-id", counterEnvelope)

					Expect(mockDataSetterB.SetInput).To(
						BeCalled(With(counterEnvelopeBytes)),
					)
				})
			})

			Context("when one firehoses subscription is unregistered", func() {
				BeforeEach(func() {
					cleanupD()
				})

				It("does not send data to that setter", func() {
					router.SendTo("some-app-id", counterEnvelope)

					Expect(mockDataSetterD.SetCalled).To(
						Not(BeCalled()),
					)
				})
			})

			Context("when one of one firehoses subscription is unregistered", func() {
				BeforeEach(func() {
					cleanupF()
				})

				It("does not send data to that setter", func() {
					router.SendTo("some-app-id", counterEnvelope)

					Expect(mockDataSetterF.SetCalled).To(
						Not(BeCalled()),
					)
				})
			})

			Describe("thread safety", func() {
				It("survives the race detector", func(done Done) {
					cleanup := router.Register(reqA, mockDataSetterA)

					go func() {
						defer close(done)
						router.SendTo("some-app-id", counterEnvelope)
					}()
					cleanup()
				})
			})
		})
	})
})
