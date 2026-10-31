-- orders 000002: the shippable order line. Shipping is admin-set
-- bookkeeping in minor units, defaulting to zero for every order the
-- checkout minted before admins weigh the parcel. The payable total is
-- subtotal + shipping, derived at read — never stored twice.
ALTER TABLE orders_order ADD COLUMN shipping_minor BIGINT NOT NULL DEFAULT 0;
