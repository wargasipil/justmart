# Pos
- Ensure compatibility POS in pharmacy mode and retail mode
- Ensure both work on postgres and sqlite
- ensure grosir price applies and reverts as cart qty crosses a tier threshold
- ensure grosir suppresses the automatic product discount but not a manual one
- ensure an order created from a grosir cart keeps the grosir price through completion, receipt and order history

# User
- ensure role in mode pharmacy shop and retail not mixed

# inventory
## Restock
- ensure can create and accept restock

## Grosir
- ensure a tier only applies to lines of its own unit (no cross-unit aggregation)
- ensure a tier priced at or above the normal price is never applied
- ensure min qty below 2 is rejected
