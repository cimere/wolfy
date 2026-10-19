# WhatsApp chatbot for a volleyball team

Historical planning brief — October 18, 2026. This describes the initial proposal and is superseded by the implemented design in [architecture.md](architecture.md).

## Goal and scope

An assistant that players can ask about practice times, match schedules, gym addresses, and organizational information.

**The standard bot replies in a private chat from its own dedicated number. It does not read the existing WhatsApp group, retrieve its history, or learn about updates automatically.** A team administrator enters official information in the knowledge base.

## Proposed components

| Component | Purpose |
| --- | --- |
| Dedicated number | The bot's WhatsApp identity; it must be verifiable through Meta's process, usually by SMS or phone call. |
| Official Meta WhatsApp Cloud API | Receives messages and sends replies without WhatsApp Web automation. |
| n8n and HTTPS webhook | The webhook receives Meta events; n8n coordinates data retrieval, memory, the OpenAI request, and the reply. |
| Always-on hosting | Hosts n8n and persistent storage, either in the cloud or on a managed server that remains reachable while a personal computer is off. |
| OpenAI API | Understands the question and formulates a response using context supplied by n8n. |
| Google Sheets | An administrator-maintained knowledge base for practice, matches, venues, organizational contacts, FAQs, and update dates. |
| Conversation memory | Persistent storage, for example PostgreSQL, with recent messages or summaries isolated by user. |

The bot runs on a server and does not need a mobile data plan to operate. The owner must maintain control of the phone number and keep it active. A data-only SIM might not support the required verification; check the carrier terms before purchasing.

## Proposed flow

```text
Player → private WhatsApp chat → Meta Cloud API
                                    ↓ HTTPS webhook
                                   n8n
                                    ├─ reads relevant data from Google Sheets
                                    ├─ retrieves that player's conversation memory
                                    └─ sends the question and context to OpenAI
                                                 ↓
                                      reply → Meta → player
                                                 ↓
                                      updates persistent memory
```

The **knowledge base is the official source**; memory helps interpret follow-up questions such as “and what time?”. Player messages must never automatically change the calendar. If information is missing or a question is ambiguous, the bot asks for clarification or refers the player to an administrator. One player's memory must never appear in another player's replies.

## Initial Meta test

1. Create a Meta app and configure the WhatsApp Cloud API with the **test number provided by Meta** before purchasing a SIM.
2. Add and verify **up to five recipient numbers** in the test dashboard: these are five private-chat users, **not a WhatsApp group**. This limit was reported in the [n8n bot setup guide](https://blog.n8n.io/whatsapp-bot/); verify it in the Meta dashboard during setup.
3. Connect the webhook and test messages in both directions; then add Sheets, OpenAI, and memory.
4. For production, register the dedicated number, meet Meta's current requirements, and use credentials suitable for continuous operation. Test credentials may expire.

## Indicative costs

**The figures below are hypothetical planning allowances, not verified commercial offers or a quote.** Assumptions: one team, a few hundred text questions per month, short context, and a low-cost model.

| Item | Planning allowance |
| --- | --- |
| n8n hosting and memory | Set aside roughly **EUR 5–15/month** for a small self-managed server; resources, backups, and maintenance may cost more. n8n Cloud has a separate subscription to verify. |
| OpenAI API | Initially set aside **EUR 1–5/month**, then measure real usage. Costs depend on model and input/output tokens, including resent memory; see the [official pricing](https://developers.openai.com/api/docs/pricing). |
| WhatsApp/Meta | Variable according to market, message category, and current terms. Check [Meta pricing](https://business.whatsapp.com/products/platform-pricing) and the account; do not assume a universal allowance of “1,000 free replies” or that messages are always free. |
| Phone number/SIM | Activation, possible monthly fees, or top-ups to keep the SIM active; verify with the carrier. No SIM offer from the earlier conversation is confirmed here. |
| Google Sheets | May use an existing account; any Workspace subscription is separate. |

Adding these assumptions gives **EUR 6–20/month for the server and OpenAI**, plus phone, possible Meta fees, and additional services. Development, support, taxes, and exchange-rate changes are excluded. The test should also measure actual cost per question.

## Checklist

- [ ] Prepare Sheets with accurate information and assign an administrator to maintain it.
- [ ] Configure a Meta account, test number, and up to five verified recipients.
- [ ] Run n8n on always-on hosting with an HTTPS webhook and protected credentials. Verify the transition from test URLs to production URLs. Reference: [n8n WhatsApp Trigger](https://docs.n8n.io/integrations/builtin/trigger-nodes/n8n-nodes-base.whatsapptrigger/).
- [ ] Store memory by user, define its lifetime and deletion process, inform participants about data processing, and limit data sent to external services.
- [ ] Test real questions, ambiguous questions, missing information, and calendar updates.
- [ ] Check chat isolation, duplicate messages, credential expiry, and Sheets/OpenAI outages; provide a human contact.
- [ ] Measure usage and costs; set spending alerts and an operational limit in the workflow.
- [ ] Before launch, verify the number, SIM terms, Meta requirements, and current prices. For proactive messages, check consent, templates, and service-window rules.
- [ ] Start a small private-chat pilot, collect feedback, and then expand to the team.
