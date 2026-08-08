# Stripe Billing setup

The application uses Stripe-hosted Checkout, a seven-day trial with a payment method collected up front, Stripe Tax, the Customer Portal, signed webhooks, and Resend for the trial-ending reminder.

## Environment

Configure the web process with:

```dotenv
STRIPE_SECRET_KEY=sk_live_...
STRIPE_WEBHOOK_SECRET=whsec_...
STRIPE_BUILDER_PRICE_ID=price_1U1UHA1uQmnGXz9tahh6px4X
STRIPE_PRO_PRICE_ID=price_1U1UHJ1uQmnGXz9t5Bqs6JVL
STRIPE_PORTAL_CONFIGURATION_ID=
RESEND_API_KEY=re_...
RESEND_FROM_EMAIL=Hypermetrics <billing@hypermetrics.dev>
```

Use sandbox keys and matching sandbox products/prices locally. Never combine a live secret key with test price IDs, or the reverse.

## Stripe Dashboard

1. In **Tax settings**, keep the default price tax behavior set to **Exclusive**. Both live Hypermetrics products use the `Software as a service (SaaS) - business use` product tax code.
2. Add only the tax registrations for jurisdictions where the business is registered to collect tax. Checkout enables automatic tax, but Stripe only collects where a matching registration exists.
3. Activate the **Customer Portal**. Enable payment-method updates, invoice history, cancellation at period end, and switching only between the Builder and Pro monthly prices. If using a non-default portal configuration, put its `bpc_...` ID in `STRIPE_PORTAL_CONFIGURATION_ID`.
4. Create a webhook endpoint at `https://YOUR_APP_HOST/stripe/webhook` for:
   - `checkout.session.completed`
   - `customer.subscription.created`
   - `customer.subscription.updated`
   - `customer.subscription.deleted`
   - `customer.subscription.paused`
   - `customer.subscription.resumed`
   - `customer.subscription.trial_will_end`
   - `invoice.payment_failed`
5. Put the endpoint signing secret in `STRIPE_WEBHOOK_SECRET`.
6. Enable Smart Retries and automatic card updates in Billing revenue recovery settings. Hypermetrics sends its own first-failure email through Resend; disable Stripe's failed-payment emails to avoid duplicate notifications.
7. Complete public business information, support details, privacy policy, terms, and cancellation policy before accepting live payments.

For local webhook testing:

```sh
stripe listen --forward-to localhost:3000/stripe/webhook
```

Use the `whsec_...` value printed by Stripe CLI as the local webhook secret.

## Resend

Verify the sending domain in Resend and set `RESEND_FROM_EMAIL` to an address on that domain. Stripe emits `customer.subscription.trial_will_end` about three days before the trial ends and `invoice.payment_failed` when collection fails. The webhook sends trial and first-failure notifications through Resend with the Stripe event ID as the email idempotency key.

## Access rules

API keys work while the subscription is `trialing`, `active`, or `past_due`. A `past_due` customer keeps access during Stripe's retry window and sees a payment-recovery banner in the dashboard. Access ends when Stripe moves the subscription to `unpaid`, `canceled`, or another non-entitled status. Builder is limited to 60 requests per minute and Pro to 600. Plan changes received from Stripe update existing API keys. A Stripe customer can receive the seven-day trial only once; later subscriptions are charged immediately.
