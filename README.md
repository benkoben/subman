# subscription-manager

A simple application that allows users to organize the subscriptions that they pay for. The application is vendor agnostic, it only cares about subscriptions that users have saved to their profile.
Therefore the user creates a subscription themselves, during this phase they typically give it a name, description, price and recurring payment interval (monthy or annually). A payment method can be attached as well but this is optional. 

Payment methods are just a way for a user to keep track of what card is used where. It holds no sensitive information about the card itself. However, an expiration date can be added if its a debid card or credit card. This is because we only care about organizing, having an expiration date allows users to keep track of affected subscriptions should the payment method expire. When a user starts a free trail from a vendor they can add this to the application as well. It is expected that a free trail will automatically convert into a regular subscription once the trail period is over, unless the user cancels the subscription. Canceling a subscription at a vendor is not the same thing as cancelling a subscription or trail in the application. They are two seperate steps, although its expected that a user cancels their subscription in the application as well to keep their collection organized. 

### Domains/subdomains

- Users - A user of the application, has its own profile where subscriptions can be organized and tracked
- Plan - A plan is a descriptive offer that a user is subscribed to or has a trail for
	- Subscription - Subscription is an active instance of a plan associated to a user
	- Trails - A trail is an active instance of a plan that automatically converts into a subscription after the first billing period
- Payment methods
	- Card
	- Paypal
