// Package commerce is the eCommerce edition: the blog module list plus the
// catalog module — native pricing, variant matrices, stock, and SKUs keyed
// to a configured product post type — and the official storefront theme in
// place of the blog theme. Install path:
//
//	go install github.com/aristorinjuang/lesstruct/cmd/lesstruct-ecommerce@latest
//
// Operators may still override any page or asset with THEME_DIR. The demo
// project's config.toml works unchanged.
package commerce

import (
	"context"
	"errors"
	"fmt"
	"log"
	"maps"
	"slices"
	"time"

	"github.com/aristorinjuang/lesstruct/internal/app"
	"github.com/aristorinjuang/lesstruct/internal/config"
	blogedition "github.com/aristorinjuang/lesstruct/internal/editions/blog"
	"github.com/aristorinjuang/lesstruct/internal/email"
	"github.com/aristorinjuang/lesstruct/internal/modules"
	kernelcontent "github.com/aristorinjuang/lesstruct/kernel/content"
	"github.com/aristorinjuang/lesstruct/kernel/legal"
	"github.com/aristorinjuang/lesstruct/kernel/pii"
	cartmodule "github.com/aristorinjuang/lesstruct/modules/cart"
	cartdomain "github.com/aristorinjuang/lesstruct/modules/cart/domain"
	cartstore "github.com/aristorinjuang/lesstruct/modules/cart/store"
	catalogmodule "github.com/aristorinjuang/lesstruct/modules/catalog"
	catalogapi "github.com/aristorinjuang/lesstruct/modules/catalog/api"
	catalogdomain "github.com/aristorinjuang/lesstruct/modules/catalog/domain"
	catalogstore "github.com/aristorinjuang/lesstruct/modules/catalog/store"
	ordersmodule "github.com/aristorinjuang/lesstruct/modules/orders"
	ordersapi "github.com/aristorinjuang/lesstruct/modules/orders/api"
	ordersdomain "github.com/aristorinjuang/lesstruct/modules/orders/domain"
	orderpdf "github.com/aristorinjuang/lesstruct/modules/orders/pdf"
	ordersstore "github.com/aristorinjuang/lesstruct/modules/orders/store"
	paymentsmodule "github.com/aristorinjuang/lesstruct/modules/payments"
	paymentsdomain "github.com/aristorinjuang/lesstruct/modules/payments/domain"
	midtransdriver "github.com/aristorinjuang/lesstruct/modules/payments/midtrans"
	paymentsstore "github.com/aristorinjuang/lesstruct/modules/payments/store"
	posttype "github.com/aristorinjuang/lesstruct/modules/posttypes-basic/domain"
	themestorefront "github.com/aristorinjuang/lesstruct/modules/theme-storefront"
)

// piiBackfillBatch bounds one pass of the startup PII backfill sweep: the
// sweep loops in batches until both stores report zero legacy rows.
const piiBackfillBatch = 100

// catalogConfigExtra projects the [catalog] block onto the public
// /api/v1/config payload the storefront and admin SPA read: product type,
// base currency, and normalized display currencies.
func catalogConfigExtra(cfg config.CatalogConfig) map[string]any {
	displays := make([]any, 0, len(cfg.DisplayCurrencies))
	for _, d := range cfg.DisplayCurrencies {
		displays = append(displays, map[string]any{"code": d.Code, "rate": d.Rate})
	}
	return map[string]any{
		"postType":          cfg.PostType,
		"currency":          cfg.Currency,
		"displayCurrencies": displays,
	}
}

// newPIICipher builds the shared PII cipher from PII_MASTER_KEY. Commerce
// cannot read or write customer PII without it, so a missing or invalid
// key fails boot closed instead of serving plaintext. Blog and core
// editions never call it and are unaffected.
func newPIICipher(cfg *config.Config) (*pii.Cipher, error) {
	key, err := config.ParsePIIMasterKey(cfg.PIIMasterKey)
	if err != nil {
		return nil, err
	}
	return pii.New(key)
}

// state carries the edition-built configuration and services. The blog
// module stack stays inside the blog edition package; only the commerce
// knobs and adopted services live here. configExtras accumulates the
// per-wire blocks (catalog, orders, payments) the admin SPA reads from the
// public config; the composition root installs the merged closure once,
// after all wires ran, instead of each wire overwriting the handler
// contract. ordersStore and paymentsStore are kept for the startup PII
// backfill sweep, which runs in Mount after module migrations.
type state struct {
	config         config.CatalogConfig
	catalogService *catalogdomain.Service
	cartService    *cartdomain.Service
	ordersService  *ordersdomain.Service
	ordersStore    *ordersstore.Store
	paymentsStore  *paymentsstore.Store
	pendingExpiry  time.Duration
	configExtras   map[string]any
}

// configExtrasBlocks returns the merged edition blocks in wire order. The
// map is always non-nil so a commerce install never serves an empty config
// extras payload.
func (s *state) configExtrasBlocks() map[string]any {
	blocks := make(map[string]any, len(s.configExtras))
	maps.Copy(blocks, s.configExtras)
	return blocks
}

// wireCatalog builds the catalog module on top of the platform services:
// store, service, module, and the public config extras the admin SPA reads.
func (s *state) wireCatalog(core *app.Core) error {
	cfg, err := config.LoadCatalog(core.Cfg)
	if err != nil {
		return err
	}
	s.config = cfg

	store := catalogstore.New(core.DB, core.Dialect)
	service := catalogdomain.NewService(store, NewCatalogMailer(core.EmailService, cfg.NotifyEmail))
	s.catalogService = service

	module := catalogmodule.New(
		catalogmodule.WithConfig(catalogapi.Config{PostType: cfg.PostType, Currency: cfg.Currency}),
		catalogmodule.WithService(service),
		catalogmodule.WithPostTypes(postTypeChecker{postTypes: core.PostTypeService}),
		catalogmodule.WithSlugResolver(publishedProductResolver(core, service)),
		catalogmodule.WithCapabilityResolver(core.Authorize),
		catalogmodule.WithLogger(modules.NewLogger(core.Logger)),
	)
	core.Candidates = append(core.Candidates, module)
	s.configExtras["catalog"] = catalogConfigExtra(cfg)

	return nil
}

// wireCart builds the cart module on top of the platform database and the
// catalog service the catalog wire adopted: store, service over the
// catalogSource seam, and module registration behind the catalog module.
func (s *state) wireCart(core *app.Core) error {
	store := cartstore.New(core.DB, core.Dialect)
	service := cartdomain.NewService(store, NewCatalogSource(s.catalogService, NewProductIdentity(core.ContentService)))
	s.cartService = service

	module := cartmodule.New(
		cartmodule.WithService(service),
		cartmodule.WithCurrency(s.config.Currency),
		cartmodule.WithLogger(modules.NewLogger(core.Logger)),
	)
	core.Candidates = append(core.Candidates, module)

	return nil
}

// wireOrders builds the orders module on top of the platform database and
// the three commerce services the earlier wires adopted: checkout claims
// the cart, titles snapshot from content, and email carries the
// notifications. The module registers behind the cart module. The manual
// payee block (when configured) rides the confirmation email and the
// guest tracking payload.
func (s *state) wireOrders(core *app.Core) error {
	ordersConfig, err := config.LoadOrders(core.Cfg)
	if err != nil {
		return err
	}
	pendingExpiry, err := ordersConfig.PendingExpiryDuration()
	if err != nil {
		return err
	}
	s.pendingExpiry = pendingExpiry
	invoiceConfig, err := config.LoadInvoice(core.Cfg)
	if err != nil {
		return err
	}
	paymentsConfig, err := config.LoadPayments(core.Cfg)
	if err != nil {
		return err
	}

	sealer, err := newPIICipher(core.Cfg)
	if err != nil {
		return err
	}

	// Checkout consent (round 6): the composition root resolved the terms and
	// privacy documents once at boot; the gate and the evidence refer to that
	// same snapshot until restart.
	consentPolicy := ordersdomain.ConsentPolicy{Required: core.CheckoutConsent}
	consentPolicy.Terms, _ = core.LegalResolution.Document(legal.PageTermsOfService)
	consentPolicy.Privacy, _ = core.LegalResolution.Document(legal.PagePrivacyPolicy)

	// The storefront banner keeps the store wording (language, currency,
	// cart); every other edition renders the neutral cookie notice.
	core.LegalNotice = legal.StoreNoticeCopy

	store := ordersstore.New(core.DB, core.Dialect, sealer)
	manualPayee := email.ManualPayee{}
	if manualPayeeAdopted(paymentsConfig.Methods, paymentsConfig.Manual) {
		manualPayee = email.ManualPayee{
			Bank:    paymentsConfig.Manual.Bank,
			Account: paymentsConfig.Manual.Account,
			Name:    paymentsConfig.Manual.Name,
		}
	}
	service := ordersdomain.NewService(
		store,
		NewCartCheckout(s.cartService),
		NewContentTitles(core.ContentService),
		NewOrderMailer(core.EmailService, manualPayee),
		NewCatalogInventory(s.catalogService, modules.NewLogger(core.Logger)),
		s.config.Currency,
		ordersdomain.WithConsentPolicy(consentPolicy),
	)

	moduleOpts := []ordersmodule.Option{
		ordersmodule.WithService(service),
		ordersmodule.WithCapabilityResolver(core.Authorize),
		ordersmodule.WithLogger(modules.NewLogger(core.Logger)),
		ordersmodule.WithSweeper(5*time.Minute, pendingExpiry),
		ordersmodule.WithInvoiceSettings(ordersdomain.InvoiceSettings{
			SellerName:    invoiceConfig.SellerName,
			SellerAddress: invoiceConfig.SellerAddress,
			SellerTaxID:   invoiceConfig.SellerTaxID,
			TaxRate:       invoiceConfig.TaxRate,
		}),
		ordersmodule.WithInvoiceSettingsSaver(func(settings ordersdomain.InvoiceSettings) error {
			return config.SaveInvoice(core.Cfg, config.InvoiceConfig{
				SellerName:    settings.SellerName,
				SellerAddress: settings.SellerAddress,
				SellerTaxID:   settings.SellerTaxID,
				TaxRate:       settings.TaxRate,
			})
		}),
		ordersmodule.WithInvoiceRenderer(orderpdf.Renderer{}),
	}
	if manualPayeeAdopted(paymentsConfig.Methods, paymentsConfig.Manual) {
		moduleOpts = append(moduleOpts, ordersmodule.WithManualPayee(ordersapi.ManualPayee{
			Bank:    paymentsConfig.Manual.Bank,
			Account: paymentsConfig.Manual.Account,
			Name:    paymentsConfig.Manual.Name,
		}))
	}

	module := ordersmodule.New(moduleOpts...)
	core.Candidates = append(core.Candidates, module)
	s.ordersService = service
	s.ordersStore = store
	s.configExtras["orders"] = map[string]any{
		"currency": s.config.Currency,
	}

	return nil
}

// wirePayments builds the payments module on top of the platform database
// and the orders service the orders wire adopted: payment records store
// over its own tables, proofs resolve and confirm through the orders
// service seam, and email carries the notices. The Midtrans Snap driver
// adopts when the environment carries a server key and the method list
// includes it; the module registers behind the orders module.
func (s *state) wirePayments(core *app.Core) error {
	paymentsConfig, err := config.LoadPayments(core.Cfg)
	if err != nil {
		return err
	}
	methods := adoptedMethods(paymentsConfig.Methods)

	var gateway paymentsdomain.Gateway
	for i := range methods {
		if methods[i] == paymentsdomain.MethodMidtrans && paymentsConfig.MidtransServerKey != "" {
			if !midtransdriver.WellFormedServerKey(paymentsConfig.MidtransServerKey, paymentsConfig.MidtransIsProduction) {
				log.Printf("commerce: MIDTRANS_SERVER_KEY does not look like a Midtrans server key (want SB-Mid-server-… on sandbox, Mid-server-… on production); gateway calls will fail upstream")
			}
			gateway = midtransdriver.New(
				paymentsConfig.MidtransServerKey,
				paymentsConfig.MidtransIsProduction,
				midtransdriver.WithSessionTTL(s.pendingExpiry),
				midtransdriver.WithFinishBase(core.BaseURL),
			)
			break
		}
	}

	sealer, err := newPIICipher(core.Cfg)
	if err != nil {
		return err
	}

	store := paymentsstore.New(core.DB, core.Dialect, sealer)
	service := paymentsdomain.NewPaymentsService(
		store,
		NewOrderSeams(s.ordersService),
		NewPaymentMailer(core.EmailService),
		gateway,
		methods,
	)

	module := paymentsmodule.New(
		paymentsmodule.WithService(service),
		paymentsmodule.WithMethods(methods),
		paymentsmodule.WithGateway(gateway),
		paymentsmodule.WithCapabilityResolver(core.Authorize),
	)
	core.Candidates = append(core.Candidates, module)
	s.paymentsStore = store
	paymentsExtra := map[string]any{
		"methods": methods,
		"gateway": service.GatewayAdopted(),
	}
	if manualPayeeAdopted(paymentsConfig.Methods, paymentsConfig.Manual) {
		paymentsExtra["manualPayee"] = map[string]any{
			"bank":    paymentsConfig.Manual.Bank,
			"account": paymentsConfig.Manual.Account,
			"name":    paymentsConfig.Manual.Name,
		}
	}
	s.configExtras["payments"] = paymentsExtra

	return nil
}

// mountWithBackfill runs the blog mount chain, then encrypts legacy
// plaintext PII rows left by pre-encryption deployments. It is split out
// so tests can exercise the commerce mount step with a stub blog mount:
// the Mount closure below must delegate here, never inline the sweep.
func (s *state) mountWithBackfill(core *app.Core, blogMount func(*app.Core) error) error {
	if blogMount != nil {
		if err := blogMount(core); err != nil {
			return err
		}
	}
	return s.backfillPII(context.Background())
}

// backfillPII encrypts legacy plaintext PII rows left by pre-encryption
// deployments and fills their blind-index columns. It runs in Mount —
// after module migrations added the index columns (Wire runs before them,
// so the columns may not exist yet) — and loops in batches until both
// stores report zero legacy rows. A corrupt row fails boot closed with
// the row identified instead of silently skipping it.
func (s *state) backfillPII(ctx context.Context) error {
	for {
		pending, err := s.ordersStore.EncryptPending(ctx, piiBackfillBatch)
		if err != nil {
			return fmt.Errorf("commerce: failed to encrypt legacy order PII: %w", err)
		}
		if pending == 0 {
			break
		}
	}
	for {
		pending, err := s.paymentsStore.EncryptPending(ctx, piiBackfillBatch)
		if err != nil {
			return fmt.Errorf("commerce: failed to encrypt legacy payment PII: %w", err)
		}
		if pending == 0 {
			break
		}
	}
	return nil
}

// adoptedMethods filters the configured method list onto the vocabulary
// the payments module implements, keeping the configured order. Unknown
// values ride through config (gateway entries can pre-list before their
// drivers exist) and drop here.
func adoptedMethods(names []string) []paymentsdomain.Method {
	methods := make([]paymentsdomain.Method, 0, len(names))
	for i := range names {
		method, ok := paymentsdomain.ParseMethod(names[i])
		if !ok {
			continue
		}
		methods = append(methods, method)
	}
	return methods
}

// manualPayeeAdopted reports whether the manual payee block rides the order
// surfaces (confirmation email and guest tracking payload): the
// [payments.manual] block must be complete AND manual must be an offered
// method. A configured block with manual absent from methods stays silent —
// advertising an unverifiable transfer would strand the buyer.
func manualPayeeAdopted(methods []string, payee config.ManualPayee) bool {
	if payee.Bank == "" || payee.Account == "" || payee.Name == "" {
		return false
	}
	return slices.Contains(methods, string(paymentsdomain.MethodManual))
}

// postTypeChecker adapts the post-type registry to the catalog module's
// existence check.
type postTypeChecker struct {
	postTypes *posttype.Service
}

// Exists reports whether the slug is a registered post type.
func (c postTypeChecker) Exists(slug string) bool {
	_, err := c.postTypes.GetBySlug(slug)
	return err == nil
}

// productContentLookup resolves a published slug to its content row. It is
// the testable seam behind the catalog slug resolver: production adapts the
// content service's published lookup, tests stub rows by slug.
type productContentLookup func(ctx context.Context, slug string) (*kernelcontent.Content, error)

// productRecordChecker reports whether a content item carries catalog data.
// Production probes the catalog service; any error reads as no record so the
// resolver falls back and the public handler surfaces the real failure.
type productRecordChecker func(ctx context.Context, contentID int) bool

// resolveProductContentID prefers a row's own catalog record, else its
// translation group root, else its own ID (so the public handler 404s on the
// product read exactly as before). Missing slugs and lookup failures resolve
// to not-found; the public surface never distinguishes them.
func resolveProductContentID(
	ctx context.Context,
	slug string,
	lookup productContentLookup,
	hasRecord productRecordChecker,
) (int, bool) {
	content, err := lookup(ctx, slug)
	if err != nil || content == nil {
		return 0, false
	}
	if hasRecord(ctx, content.ID) {
		return content.ID, true
	}
	if content.TranslationGroupID != nil {
		if hasRecord(ctx, *content.TranslationGroupID) {
			return *content.TranslationGroupID, true
		}
	}
	return content.ID, true
}

// publishedProductResolver adapts the content service's published lookup to
// the catalog module's slug seam: a published product resolves to its
// content ID; drafts, missing items, and lookup failures resolve to
// not-found (the public surface must never distinguish them). Translated
// rows without their own catalog record fall back to the group root so the
// public payload resolves; translated rows with their own record keep it.
func publishedProductResolver(core *app.Core, catalog *catalogdomain.Service) catalogapi.ProductSlugResolver {
	lookup := func(ctx context.Context, slug string) (*kernelcontent.Content, error) {
		if core == nil || core.ContentService == nil {
			return nil, errors.New("content service unavailable")
		}
		return core.ContentService.GetPublishedBySlugAny(ctx, slug)
	}
	hasRecord := func(ctx context.Context, contentID int) bool {
		if catalog == nil {
			return false
		}
		_, err := catalog.GetProduct(ctx, contentID)
		// Fall back only on a genuinely missing record. Any other failure
		// reads as present so the resolver keeps the row's own ID and the
		// public handler surfaces the real error instead of silently
		// serving a different product.
		return err == nil || !errors.Is(err, catalogdomain.ErrProductNotFound)
	}
	return func(ctx context.Context, slug string) (int, bool) {
		return resolveProductContentID(ctx, slug, lookup, hasRecord)
	}
}

// Edition returns the commerce edition: the full blog composition plus the
// catalog, cart, and orders modules, with the storefront theme registered in
// place of the default blog theme. The blog hooks run first (their modules
// adopt the platform services), then the catalog wire adds the commerce
// stack and accumulates its config extras, the cart wire adds the
// token-keyed carts priced from the catalog service, and the orders wire
// adds checkout over the cart plus the admin order surface. The merged
// config extras install once after every wire ran.
func Edition() app.Edition {
	base := blogedition.EditionWithTheme(themestorefront.New())
	s := &state{configExtras: map[string]any{}}

	base.Name = "commerce"
	blogWire, blogMount := base.Wire, base.Mount
	base.Wire = func(core *app.Core) error {
		if blogWire != nil {
			if err := blogWire(core); err != nil {
				return err
			}
		}
		if err := s.wireCatalog(core); err != nil {
			return err
		}
		if err := s.wireCart(core); err != nil {
			return err
		}
		if err := s.wireOrders(core); err != nil {
			return err
		}
		if err := s.wirePayments(core); err != nil {
			return err
		}
		core.Handlers().ConfigExtras = s.configExtrasBlocks
		return nil
	}
	base.Mount = func(core *app.Core) error {
		return s.mountWithBackfill(core, blogMount)
	}
	return base
}
