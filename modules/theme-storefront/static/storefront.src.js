/* Lesstruct storefront: cart, checkout, and guest order tracking over the
   public JSON APIs. The server renders the shells; everything below hydrates
   them so the storefront pages stay static and cacheable.
   Readable source of truth — edit this file, then regenerate with:
       make js       ->  static/storefront.js */
(() => {
	"use strict";

	const API = "/api/v1/public";
	const TOKEN_KEY = "lesstruct_cart_token";
	const ORDER_KEY = "lesstruct_last_order";
	const CURRENCY_KEY = "lesstruct_currency";
	// Keep in sync with modules/orders/domain/money.go: zero-decimal plus
	// two-decimal currencies only; three-decimal currencies are unsupported.
	const ZERO_DECIMAL = new Set(["IDR", "JPY", "KRW", "VND"]);

	let baseCurrency = "IDR";
	let displayOptions = [];
	let currentDisplay = null;

	function readConsentCookie() {
		try {
			const m = document.cookie.match(/(?:^|;\s*)lesstruct_consent=([^;]*)/);
			return m ? decodeURIComponent(m[1]).trim() : "";
		} catch (_) {
			return "";
		}
	}

	// hasAuthHint reports whether the login hint cookie is present. Anonymous
	// loads skip the profile prefill entirely so no 401 probe is issued.
	function hasAuthHint() {
		try {
			return /(?:^|;\s*)lesstruct_auth=/.test(document.cookie);
		} catch (_) {
			return false;
		}
	}

	// secureSuffix returns "; Secure" exactly when the page itself is HTTPS,
	// so preference cookies stay off plain HTTP without breaking local
	// development. Deletions need it too: a Secure cookie survives a
	// deletion that omits Secure.
	function secureSuffix() {
		return window.location.protocol === "https:" ? "; Secure" : "";
	}

	function writeConsentCookie(value) {
		try {
			document.cookie = "lesstruct_consent=" + encodeURIComponent(value) + "; Path=/; Max-Age=31536000; SameSite=Lax" + secureSuffix();
		} catch (_) {
			/* cookie storage unavailable: the banner simply will not persist */
		}
	}

	function canPersistPreferences() {
		return readConsentCookie() === "accepted";
	}

	function readDisplayCurrency() {
		if (!canPersistPreferences()) {
			return "";
		}
		try {
			return localStorage.getItem(CURRENCY_KEY) || "";
		} catch (_) {
			return "";
		}
	}

	function writeDisplayCurrency(code) {
		if (!canPersistPreferences()) {
			return;
		}
		try {
			if (code) localStorage.setItem(CURRENCY_KEY, code);
			else localStorage.removeItem(CURRENCY_KEY);
		} catch (_) {
			/* storage unavailable: the toggle simply will not persist */
		}
	}

	function displayRateFor(code) {
		const upper = (code || "").toUpperCase();
		for (const opt of displayOptions) {
			if ((opt.code || "").toUpperCase() === upper) return Number(opt.rate) || 0;
		}
		return 0;
	}

	function convertMinor(minor, base, display) {
		if (!display || display.toUpperCase() === (base || "").toUpperCase()) {
			return { currency: base, minor };
		}
		const rate = displayRateFor(display);
		if (!(rate > 0)) return { currency: base, minor };
		// rate is display-major per base-minor (e.g. USD per IDR minor unit).
		// Scale to display-minor: no-op for zero-decimal display currencies,
		// cents otherwise.
		const scale = ZERO_DECIMAL.has(display.toUpperCase()) ? 1 : 100;
		return { currency: display, minor: Math.round(Number(minor || 0) * rate * scale) };
	}

	function activeDisplayCurrency() {
		if (currentDisplay && currentDisplay.toUpperCase() !== baseCurrency.toUpperCase() && displayRateFor(currentDisplay) > 0) {
			return currentDisplay.toUpperCase();
		}
		return baseCurrency;
	}

	const lang = (document.documentElement.lang || "en").toLowerCase();
	const isID = lang.startsWith("id");

	const copy = {
		emptyCart: isID ? "Keranjangmu masih kosong." : "Your cart is empty.",
		emptyCartHint: isID ? "Mulai dari koleksi merch resmi kami." : "Start with the official merch collection.",
		shop: isID ? "Belanja sekarang" : "Shop the collection",
		remove: isID ? "Hapus" : "Remove",
		unavailable: isID ? "Tidak tersedia" : "Unavailable",
		inStock: isID ? "Tersedia" : "In stock",
		lowStock: isID ? "Stok terbatas" : "Low stock",
		outOfStock: isID ? "Habis" : "Sold out",
		selectSize: isID ? "Pilih ukuran dulu." : "Choose a size first.",
		added: isID ? "Ditambahkan ke keranjang." : "Added to cart.",
		oneSize: isID ? "Satu ukuran" : "One size",
		loading: isID ? "Memuat…" : "Loading…",
		genericError: isID ? "Terjadi kesalahan. Coba lagi." : "Something went wrong. Please try again.",
		productUnavailable: isID ? "Produk tidak tersedia." : "This product is unavailable.",
		emptyCheckout: isID ? "Keranjangmu kosong." : "Your cart is empty.",
		unavailableCheckout: isID ? "Hapus item yang tidak tersedia sebelum checkout." : "Remove unavailable items before checkout.",
		consentRequired: isID ? "Setujui Syarat & Ketentuan dan Kebijakan Privasi untuk melanjutkan." : "Please agree to the Terms of Service and Privacy Policy to continue.",
		orderLookup: isID ? "Pesanan tidak ditemukan. Periksa nomor dan email." : "Order not found. Check the number and email.",
		submitted: isID ? "Bukti pembayaran diterima dan menunggu verifikasi." : "Payment proof received and awaiting verification.",
		noPayment: isID ? "Pembayaran diatur langsung oleh toko. Cek email untuk instruksi." : "Payment is arranged by the store. Check your email for instructions.",
		methodManual: isID ? "Transfer manual" : "Manual transfer",
		methodMidtrans: isID ? "Kartu / e-wallet" : "Card / e-wallet",
		reference: isID ? "Referensi / ID transaksi" : "Reference / transaction ID",
		referenceHint: isID ? "Tempel bukti transfer atau ID transaksi di sini." : "Paste your transfer reference or transaction ID here.",
		submitProof: isID ? "Kirim Bukti" : "Submit proof",
		payNow: isID ? "Bayar Sekarang" : "Pay now",
		placed: isID ? "Dipesan" : "Placed",
		paid: isID ? "Dibayar" : "Paid",
		packed: isID ? "Dikemas" : "Packed",
		shipped: isID ? "Dikirim" : "Shipped",
		delivered: isID ? "Diterima" : "Delivered",
		orderNumber: isID ? "Nomor pesanan" : "Order number",
		orderDate: isID ? "Tanggal" : "Date",
		items: isID ? "Item" : "Items",
		subtotal: isID ? "Subtotal" : "Subtotal",
		shipping: isID ? "Pengiriman" : "Shipping",
		total: isID ? "Total" : "Total",
		address: isID ? "Alamat pengiriman" : "Shipping address",
		status: isID ? "Status" : "Status",
	};

	const statusLabels = {
		pending: isID ? "Menunggu pembayaran" : "Awaiting payment",
		paid: isID ? "Dibayar" : "Paid",
		packed: isID ? "Dikemas" : "Packed",
		shipped: isID ? "Dikirim" : "Shipped",
		delivered: isID ? "Diterima" : "Delivered",
		cancelled: isID ? "Dibatalkan" : "Cancelled",
		refunded: isID ? "Dikembalikan" : "Refunded",
		expired: isID ? "Kedaluwarsa" : "Expired",
	};

	const timeline = [
		{ status: "pending", label: copy.placed },
		{ status: "paid", label: copy.paid },
		{ status: "packed", label: copy.packed },
		{ status: "shipped", label: copy.shipped },
		{ status: "delivered", label: copy.delivered },
	];

	const statusOrder = { pending: 0, paid: 1, packed: 2, shipped: 3, delivered: 4 };

	let badgeEl = null;

	// ---------- helpers ----------

	function el(tag, className, text) {
		const node = document.createElement(tag);
		if (className) node.className = className;
		if (text !== undefined) node.textContent = text;
		return node;
	}

	function formatMoney(currency, amount) {
		const code = (currency || "USD").toUpperCase();
		const zero = ZERO_DECIMAL.has(code);
		const value = zero ? Number(amount || 0) : Number(amount || 0) / 100;
		try {
			return new Intl.NumberFormat(isID ? "id-ID" : "en-US", {
				style: "currency",
				currency: code,
				minimumFractionDigits: zero ? 0 : 2,
				maximumFractionDigits: zero ? 0 : 2,
			}).format(value);
		} catch (_) {
			return code + " " + value.toFixed(zero ? 0 : 2);
		}
	}

	function formatDate(raw) {
		const date = new Date(raw);
		if (Number.isNaN(date.getTime())) return "";
		return new Intl.DateTimeFormat(isID ? "id-ID" : "en-US", {
			year: "numeric",
			month: "long",
			day: "numeric",
		}).format(date);
	}

	function readToken() {
		try {
			return localStorage.getItem(TOKEN_KEY) || "";
		} catch (_) {
			return "";
		}
	}

	function writeToken(token) {
		try {
			if (token) localStorage.setItem(TOKEN_KEY, token);
			else localStorage.removeItem(TOKEN_KEY);
		} catch (_) {
			/* storage unavailable: the cart simply will not persist */
		}
	}

	function readLastOrder() {
		try {
			return JSON.parse(sessionStorage.getItem(ORDER_KEY) || "null");
		} catch (_) {
			return null;
		}
	}

	function writeLastOrder(value) {
		try {
			sessionStorage.setItem(ORDER_KEY, JSON.stringify(value));
		} catch (_) {
			/* best effort */
		}
	}

	function showError(target, message) {
		if (!target) return;
		target.textContent = message || copy.genericError;
		target.hidden = false;
	}

	function clearError(target) {
		if (!target) return;
		target.textContent = "";
		target.hidden = true;
	}

	async function request(path, options = {}) {
		const method = options.method || "GET";
		const headers = { Accept: "application/json" };
		if (options.body !== undefined) headers["Content-Type"] = "application/json";
		const token = options.token !== undefined ? options.token : readToken();
		if (options.cart && token) headers["X-Cart-Token"] = token;
		const response = await fetch(API + path, {
			method,
			headers,
			body: options.body === undefined ? undefined : JSON.stringify(options.body),
		});
		const payload = await response.json().catch(() => null);
		if (!response.ok) {
			const error = new Error((payload && payload.error && payload.error.message) || copy.genericError);
			error.status = response.status;
			error.code = payload && payload.error ? payload.error.code : "";
			throw error;
		}
		return payload ? payload.data : null;
	}

	async function getCart() {
		return request("/cart", { cart: true });
	}

	async function createCart() {
		const data = await request("/cart", { method: "POST" });
		if (data && data.token) writeToken(data.token);
		return data ? data.cart : null;
	}

	async function addLine(productContentId, variantId, quantity) {
		const body = { productContentId, quantity };
		if (variantId !== undefined && variantId !== null) body.variantId = variantId;
		if (!readToken()) await createCart();
		try {
			return await request("/cart/items", { method: "POST", cart: true, body });
		} catch (error) {
			if (error.status !== 401) throw error;
			writeToken("");
			await createCart();
			return request("/cart/items", { method: "POST", cart: true, body });
		}
	}

	function setBadge(count) {
		if (!badgeEl) return;
		const value = Number(count) || 0;
		badgeEl.textContent = String(value);
		badgeEl.hidden = value <= 0;
	}

	async function refreshBadge() {
		badgeEl = document.getElementById("cart-count");
		if (!badgeEl) return;
		if (!readToken()) {
			setBadge(0);
			return;
		}
		try {
			const cart = await getCart();
			setBadge(cart.itemCount);
		} catch (error) {
			if (error.status === 401) writeToken("");
			setBadge(0);
		}
	}

	// ---------- product data (deduplicated per page) ----------

	const productCache = new Map();

	function getProduct(slug) {
		if (!productCache.has(slug)) {
			const pending = request("/catalog/products/" + encodeURIComponent(slug)).catch((error) => {
				productCache.delete(slug);
				throw error;
			});
			productCache.set(slug, pending);
		}
		return productCache.get(slug);
	}

	function renderPrice(target, currency, price, salePrice) {
		if (!target) return;
		const display = activeDisplayCurrency();
		const base = (currency || baseCurrency).toUpperCase();
		const converted = convertMinor(price, base, display);
		const convertedSale = salePrice === undefined || salePrice === null ? null : convertMinor(salePrice, base, display).minor;
		target.textContent = "";
		target.append(el("span", "price__value", formatMoney(converted.currency, converted.minor)));
		if (convertedSale !== null) {
			target.append(el("span", "price__tag", isID ? "Promo" : "Sale"));
		}
	}

	async function initCurrencyToggle() {
		const select = document.getElementById("currency-select");
		try {
			const res = await fetch("/api/v1/config", { headers: { Accept: "application/json" } });
			const payload = await res.json().catch(() => null);
			const catalog = payload && payload.data && payload.data.catalog;
			if (catalog) {
				if (catalog.currency) baseCurrency = String(catalog.currency).toUpperCase();
				if (Array.isArray(catalog.displayCurrencies)) displayOptions = catalog.displayCurrencies;
			}
		} catch (_) {
			/* config unavailable: toggle stays hidden, base currency applies */
		}
		const stored = readDisplayCurrency();
		if (stored && displayRateFor(stored) > 0) currentDisplay = stored.toUpperCase();
		else currentDisplay = baseCurrency;
		if (!select) return;
		if (!displayOptions.length) {
			select.hidden = true;
			return;
		}
		select.textContent = "";
		const baseOpt = document.createElement("option");
		baseOpt.value = baseCurrency;
		baseOpt.textContent = baseCurrency;
		select.append(baseOpt);
		for (const opt of displayOptions) {
			const code = String(opt.code || "").toUpperCase();
			if (!code || code === baseCurrency) continue;
			const option = document.createElement("option");
			option.value = code;
			option.textContent = code;
			select.append(option);
		}
		select.value = currentDisplay;
		select.hidden = false;
		select.addEventListener("change", async () => {
			currentDisplay = select.value || baseCurrency;
			writeDisplayCurrency(currentDisplay === baseCurrency ? "" : currentDisplay);
			refreshDisplayPrices();
		});
	}

	function initLanguageSelect() {
		const select = document.getElementById("language-select");
		if (!select) return;
		if (!select.options.length) {
			select.hidden = true;
			return;
		}
		select.hidden = false;
		select.addEventListener("change", () => {
			if (select.value) window.location.href = select.value;
		});
	}

	function initCookieBanner() {
		const banner = document.getElementById("cookie-banner");
		if (!banner) return;
		const choice = readConsentCookie();
		if (choice === "accepted" || choice === "declined") {
			banner.hidden = true;
			return;
		}
		banner.hidden = false;
		const accept = document.getElementById("cookie-accept");
		const decline = document.getElementById("cookie-decline");
		if (accept) accept.addEventListener("click", () => {
			writeConsentCookie("accepted");
			if (currentDisplay && currentDisplay !== baseCurrency) writeDisplayCurrency(currentDisplay);
			banner.hidden = true;
		});
		if (decline) decline.addEventListener("click", () => {
			writeConsentCookie("declined");
			banner.hidden = true;
			try {
				localStorage.removeItem(CURRENCY_KEY);
			} catch (_) {
				/* storage unavailable: nothing to clear */
			}
			try {
				document.cookie = "lesstruct_lang=; Path=/; Max-Age=0; SameSite=Lax" + secureSuffix();
			} catch (_) {
				/* cookie deletion best effort */
			}
			currentDisplay = baseCurrency;
			const select = document.getElementById("currency-select");
			if (select) select.value = baseCurrency;
			refreshDisplayPrices();
		});
	}

	function renderCardStock(target, product) {
		if (!target) return;
		const inStock = Boolean(product.inStock);
		const low = inStock && Boolean(product.lowStock);
		target.textContent = !inStock ? copy.outOfStock : low ? copy.lowStock : "";
		target.classList.toggle("is-low", low);
		target.classList.toggle("is-out", !inStock);
		target.hidden = inStock && !low;
	}

	async function hydrateCardPrices() {
		const targets = Array.from(document.querySelectorAll("[data-price][data-product-slug]")).filter(
			(target) => !target.hasAttribute("data-price-main"),
		);
		await Promise.all(
			targets.map(async (target) => {
				try {
					const product = await getProduct(target.getAttribute("data-product-slug"));
					renderPrice(target, product.currency, product.price, product.salePrice);
					const card = target.closest(".product-card");
					renderCardStock(card ? card.querySelector("[data-stock]") : null, product);
				} catch (_) {
					target.remove();
				}
			}),
		);
	}

	// refreshDisplayPrices re-renders every price on the page. The currency
	// toggle calls it once the config resolves so an early base-currency
	// render converts without a reload.
	function refreshDisplayPrices() {
		hydrateCardPrices().catch(() => {});
		const page = document.querySelector("[data-product-page]");
		if (page && page._refreshDisplay) page._refreshDisplay();
	}

	// ---------- product page ----------

	function sortedAxes(product) {
		// Preserve the operator-configured axis order: alphabetical sorting
		// would scramble semantic orders such as Size before Color.
		const options = product.options || {};
		return Object.keys(options);
	}

	function axisValues(product, axis) {
		// Preserve the configured value order (S, M, L, XL — not alphabetical).
		const values = (product.options || {})[axis] || [];
		return [...values];
	}

	async function initProductPage() {
		const page = document.querySelector("[data-product-page]");
		if (!page) return;

		const slug = page.getAttribute("data-product-slug") || "";
		const priceEl = page.querySelector(".product-info [data-price]");
		const axesRoot = page.querySelector("[data-variant-axes]");
		const matrixRoot = page.querySelector("[data-variant-matrix]");
		const stockEl = document.getElementById("product-stock");
		const sizesEl = document.getElementById("product-sizes");
		const skuRow = document.getElementById("product-sku-row");
		const skuEl = document.getElementById("product-sku");
		const addBtn = document.getElementById("add-to-cart");
		const buyBtn = document.getElementById("buy-now");
		const qtyInput = document.getElementById("product-quantity");
		const errorEl = document.getElementById("product-error");
		const noteEl = document.getElementById("cart-note");
		const mainImage = document.getElementById("product-main-image");
		const defaultImageSrc = mainImage && mainImage.tagName === "IMG" ? mainImage.getAttribute("src") || "" : "";

		if (!slug) return;

		let product = null;
		let selectedVariant = null;
		let variants = [];
		let axes = [];
		let selected = {};

		function setStock(text, state) {
			if (!stockEl) return;
			stockEl.textContent = text;
			stockEl.className = "product-stock" + (state ? " is-" + state : "");
		}

		function setEnabled(enabled) {
			if (addBtn) addBtn.disabled = !enabled;
			if (buyBtn) buyBtn.disabled = !enabled;
		}

		function setSKU(sku) {
			const value = (sku || "").trim();
			if (!skuRow || !skuEl) return;
			if (!value) {
				skuRow.hidden = true;
				return;
			}
			skuEl.textContent = value;
			skuRow.hidden = false;
		}

		function matchesExcept(variant, exceptAxis) {
			const options = variant.options || {};
			for (const axis of axes) {
				if (axis === exceptAxis) continue;
				if (selected[axis] !== undefined && options[axis] !== selected[axis]) return false;
			}
			return true;
		}

		function findExact() {
			if (Object.keys(selected).length !== axes.length) return null;
			for (const variant of variants) {
				const options = variant.options || {};
				let ok = true;
				for (const axis of axes) {
					if (options[axis] !== selected[axis]) {
						ok = false;
						break;
					}
				}
				if (ok) return variant;
			}
			return null;
		}

		function refreshMatrix() {
			if (!axesRoot) return;
			for (const axis of axes) {
				const container = axesRoot.querySelector(`[data-axis="${CSS.escape(axis)}"]`);
				if (!container) continue;
				for (const chip of container.querySelectorAll(".variant-chip")) {
					const value = chip.dataset.value || "";
					const candidates = variants.filter((v) => (v.options || {})[axis] === value && matchesExcept(v, axis));
					const available = candidates.some((v) => v.inStock);
					chip.disabled = !available;
					if (!available) chip.title = copy.outOfStock;
					else chip.removeAttribute("title");
					const active = selected[axis] === value;
					chip.classList.toggle("is-active", active);
					chip.setAttribute("aria-pressed", active ? "true" : "false");
				}
			}
		}

		function selectVariant(variant) {
			selectedVariant = variant || null;
			if (selectedVariant && Object.keys(selected).length === axes.length) {
				const inStock = Boolean(selectedVariant.inStock);
				renderPrice(priceEl, product.currency, selectedVariant.price, product.salePrice);
				setStock(inStock ? (product.lowStock ? copy.lowStock : copy.inStock) : copy.outOfStock, inStock ? (product.lowStock ? "low" : "in") : "out");
				if (mainImage && mainImage.tagName === "IMG") {
					mainImage.src = selectedVariant.imageUrl || defaultImageSrc || mainImage.src;
				}
				setSKU(selectedVariant.sku || product.sku || "");
			} else {
				// Partial selection or invalid combination: never show a
				// previous exact match's price, photo, or SKU.
				renderPrice(priceEl, product.currency, product.price, product.salePrice);
				setStock(product.inStock ? (product.lowStock ? copy.lowStock : copy.inStock) : copy.outOfStock, product.inStock ? (product.lowStock ? "low" : "in") : "out");
				if (mainImage && mainImage.tagName === "IMG" && defaultImageSrc) {
					mainImage.src = defaultImageSrc;
				}
				setSKU(product.sku || "");
			}
			refreshMatrix();
			setEnabled(Boolean(selectedVariant && selectedVariant.inStock) || (!variants.length && product.inStock));
		}

		function showProductError(message) {
			showError(errorEl, message);
		}

		function refreshDisplay() {
			if (!product) return;
			if (selectedVariant) renderPrice(priceEl, product.currency, selectedVariant.price, product.salePrice);
			else renderPrice(priceEl, product.currency, product.price, product.salePrice);
		}
		page._refreshDisplay = refreshDisplay;

		try {
			product = await getProduct(slug);
		} catch (_) {
			setStock(copy.productUnavailable, "out");
			setEnabled(false);
			setSKU("");
			return;
		}

		if (product.currency) baseCurrency = String(product.currency).toUpperCase();
		renderPrice(priceEl, product.currency, product.price, product.salePrice);
		setSKU(product.sku || "");
		variants = product.variants || [];
		axes = sortedAxes(product);

		if (!variants.length || !axes.length) {
			if (matrixRoot) matrixRoot.hidden = true;
			if (sizesEl) sizesEl.textContent = copy.oneSize;
			setStock(product.inStock ? (product.lowStock ? copy.lowStock : copy.inStock) : copy.outOfStock, product.inStock ? (product.lowStock ? "low" : "in") : "out");
			setEnabled(product.inStock);
		} else if (axes.length === 1) {
			const axis = axes[0];
			const values = axisValues(product, axis);
			if (sizesEl) sizesEl.textContent = values.join(" · ");
			if (axesRoot) {
				axesRoot.textContent = "";
				const row = el("div", "variant-chips");
				row.setAttribute("role", "group");
				if (values.length > 1) {
					const note = el("p", "variant-note", isID ? "Pilih ukuran yang tersedia." : "Pick an available size.");
					axesRoot.after(note);
				}
				for (const value of values) {
					const matches = variants.filter((variant) => (variant.options || {})[axis] === value);
					const available = matches.some((variant) => variant.inStock);
					const button = el("button", "variant-chip", value);
					button.type = "button";
					button.dataset.value = value;
					button.setAttribute("aria-pressed", "false");
					button.disabled = !available;
					if (!available) button.title = copy.outOfStock;
					button.addEventListener("click", () => {
						selected = { [axis]: value };
						const chosen = matches.find((variant) => variant.inStock) || null;
						for (const chip of row.querySelectorAll(".variant-chip")) {
							const active = chip.dataset.value === value;
							chip.classList.toggle("is-active", active);
							chip.setAttribute("aria-pressed", active ? "true" : "false");
						}
						selectVariant(chosen);
						clearError(errorEl);
					});
					row.append(button);
				}
				axesRoot.append(row);
			}
			setStock(product.inStock ? (product.lowStock ? copy.lowStock : copy.inStock) : copy.outOfStock, product.inStock ? (product.lowStock ? "low" : "in") : "out");
			setEnabled(false);
		} else {
			const sizesLabel = document.getElementById("product-sizes-label");
			if (sizesLabel) sizesLabel.textContent = isID ? "Varian" : "Variants";
			if (sizesEl) {
				const summary = axes.map((axis) => axisValues(product, axis).join(", ")).join(" · ");
				sizesEl.textContent = summary || copy.oneSize;
			}
			if (axesRoot) {
				axesRoot.textContent = "";
				for (const axis of axes) {
					const section = el("div", "variant-axis");
					section.dataset.axis = axis;
					section.append(el("p", "variant-axis__label", axis));
					const row = el("div", "variant-chips");
					row.setAttribute("role", "group");
					row.setAttribute("aria-label", axis);
					for (const value of axisValues(product, axis)) {
						const button = el("button", "variant-chip", value);
						button.type = "button";
						button.dataset.value = value;
						button.setAttribute("aria-pressed", "false");
						button.addEventListener("click", () => {
							if (selected[axis] === value) delete selected[axis];
							else selected[axis] = value;
							const exact = findExact();
							selectVariant(exact);
							if (!exact && Object.keys(selected).length === axes.length) {
								setStock(copy.outOfStock, "out");
								setEnabled(false);
							} else if (!exact) {
								setStock(product.inStock ? (product.lowStock ? copy.lowStock : copy.inStock) : copy.outOfStock, product.inStock ? (product.lowStock ? "low" : "in") : "out");
								setEnabled(false);
							}
							clearError(errorEl);
						});
						row.append(button);
					}
					section.append(row);
					axesRoot.append(section);
				}
				refreshMatrix();
			}
			setStock(product.inStock ? (product.lowStock ? copy.lowStock : copy.inStock) : copy.outOfStock, product.inStock ? (product.lowStock ? "low" : "in") : "out");
			setEnabled(false);
		}

		const galleryThumbs = page.querySelectorAll(".product-gallery__thumb");
		const mobileQuery = window.matchMedia("(max-width: 767px)");
		for (const thumb of galleryThumbs) {
			thumb.addEventListener("click", () => {
				if (!mainImage || mainImage.tagName !== "IMG") return;
				const large = thumb.getAttribute("data-image");
				const medium = thumb.getAttribute("data-image-medium");
				mainImage.src = (mobileQuery.matches && medium) || large || mainImage.src;
				const srcset = thumb.getAttribute("data-srcset");
				if (srcset) mainImage.setAttribute("srcset", srcset);
				else mainImage.removeAttribute("srcset");
				// While the mobile source matches, the browser renders from
				// the <source> and ignores img attribute changes — and some
				// mobile engines do not re-run picture selection on a bare
				// srcset mutation. Re-inserting the source node is a
				// guaranteed "sources changed" signal that forces
				// re-selection in every engine.
				const picture = mainImage.closest ? mainImage.closest("picture") : null;
				const source = picture ? picture.querySelector("source") : null;
				if (source) {
					if (medium) source.setAttribute("srcset", medium);
					else source.removeAttribute("srcset");
					picture.insertBefore(source, mainImage);
				}
				for (const other of galleryThumbs) other.classList.toggle("is-active", other === thumb);
			});
		}

		async function addToCart(redirect) {
			clearError(errorEl);
			if (variants.length && !selectedVariant) {
				showProductError(copy.selectSize);
				return;
			}
			const quantity = Math.max(1, Math.min(99, parseInt(qtyInput && qtyInput.value, 10) || 1));
			if (qtyInput) qtyInput.value = String(quantity);
			setEnabled(false);
			try {
				const cart = await addLine(product.contentId, selectedVariant ? selectedVariant.id : null, quantity);
				setBadge(cart.itemCount);
				if (redirect) {
					window.location.href = "/cart";
					return;
				}
				if (noteEl) noteEl.hidden = false;
				setStock(copy.added, "in");
			} catch (error) {
				showProductError(error.message);
				setEnabled(Boolean(!variants.length ? product.inStock : selectedVariant && selectedVariant.inStock));
			}
		}

		if (addBtn) addBtn.addEventListener("click", () => addToCart(false));
		if (buyBtn) buyBtn.addEventListener("click", () => addToCart(true));
		if (qtyInput) {
			qtyInput.addEventListener("change", () => {
				qtyInput.value = String(Math.max(1, Math.min(99, parseInt(qtyInput.value, 10) || 1)));
			});
		}
	}

	// ---------- cart page ----------

	function emptyCartPanel() {
		const wrap = el("div", "cart-empty");
		wrap.append(el("h2", "", copy.emptyCart), el("p", "", copy.emptyCartHint));
		const link = el("a", "btn", copy.shop);
		link.href = "/";
		wrap.append(link);
		return wrap;
	}

	function renderSummary(cart) {
		const subtotal = document.querySelector("[data-cart-subtotal]");
		const total = document.querySelector("[data-cart-total]");
		const value = formatMoney(cart.currency, cart.subtotal);
		if (subtotal) subtotal.textContent = value;
		if (total) total.textContent = value;
	}

	function setCheckoutState(cart) {
		const link = document.getElementById("checkout-link");
		const errorEl = document.getElementById("cart-error");
		const unavailable = Boolean(cart && cart.lines.some((line) => !line.available));
		if (link) link.hidden = !cart || !cart.itemCount || unavailable;
		if (unavailable) {
			showError(errorEl, copy.unavailableCheckout);
		} else if (errorEl && errorEl.textContent === copy.unavailableCheckout) {
			clearError(errorEl);
		}
	}

	async function initCartPage() {
		const page = document.querySelector("[data-cart-page]");
		if (!page) return;
		const list = document.getElementById("cart-lines");
		const errorEl = document.getElementById("cart-error");
		const summary = document.getElementById("cart-summary");
		const setSummary = (visible) => {
			if (summary) summary.hidden = !visible;
		};
		if (!list) return;

		list.textContent = "";
		if (!readToken()) {
			list.append(emptyCartPanel());
			setSummary(false);
			setCheckoutState(null);
			return;
		}

		let cart = null;
		try {
			cart = await getCart();
		} catch (error) {
			if (error.status === 401) {
				writeToken("");
				list.append(emptyCartPanel());
				setSummary(false);
				setCheckoutState(null);
				return;
			}
			showError(errorEl, error.message);
			list.textContent = "";
			setSummary(false);
			return;
		}

		setBadge(cart.itemCount);
		renderSummary(cart);
		setCheckoutState(cart);
		setSummary(cart.lines.length > 0);

		if (!cart.lines.length) {
			list.append(emptyCartPanel());
			return;
		}

		for (const line of cart.lines) {
			list.append(buildCartLine(cart, line, list, errorEl));
		}
	}

	function buildCartLine(cart, line, list, errorEl) {
		const row = el("article", "cart-line" + (line.available ? "" : " is-unavailable"));

		const media = el("div", "cart-line__media");
		if (line.imageUrl) {
			const image = el("img");
			image.src = line.imageUrl;
			image.alt = "";
			image.loading = "lazy";
			media.append(image);
		} else {
			media.append(el("div", "cart-line__placeholder", (line.title || "?").slice(0, 1).toUpperCase()));
		}

		const body = el("div", "cart-line__body");
		const title = el("a", "cart-line__title", line.title || "Item #" + line.productContentId);
		if (line.slug) title.href = "/" + encodeURIComponent(line.slug);
		body.append(title);
		const meta = el("p", "cart-line__meta");
		const bits = [];
		if (line.sku) bits.push(line.sku);
		if (!line.available) bits.push(copy.unavailable);
		else if (!line.inStock) bits.push(copy.outOfStock);
		meta.textContent = bits.join(" · ");
		body.append(meta);

		if (line.available) {
			const controls = el("div", "cart-line__controls");
			const qty = el("div", "qty-control");
			const minus = el("button", "", "−");
			minus.type = "button";
			minus.disabled = line.quantity <= 1;
			minus.setAttribute("aria-label", isID ? "Kurangi jumlah" : "Decrease quantity");
			const count = el("span", "", String(line.quantity));
			const plus = el("button", "", "+");
			plus.type = "button";
			plus.disabled = line.quantity >= 99;
			plus.setAttribute("aria-label", isID ? "Tambah jumlah" : "Increase quantity");
			const remove = el("button", "cart-line__remove", copy.remove);
			remove.type = "button";
			controls.append(qty, remove);
			qty.append(minus, count, plus);
			body.append(controls);

			async function update(quantity) {
				clearError(errorEl);
				minus.disabled = true;
				plus.disabled = true;
				remove.disabled = true;
				try {
					const next = await request("/cart/items/" + line.id, {
						method: "PATCH",
						cart: true,
						body: { quantity },
					});
					renderCartResponse(next, list, errorEl);
				} catch (error) {
					showError(errorEl, error.message);
					minus.disabled = quantity <= 1;
					plus.disabled = quantity >= 99;
					remove.disabled = false;
				}
			}

			async function drop() {
				clearError(errorEl);
				remove.disabled = true;
				try {
					const next = await request("/cart/items/" + line.id, { method: "DELETE", cart: true });
					renderCartResponse(next, list, errorEl);
				} catch (error) {
					showError(errorEl, error.message);
					remove.disabled = false;
				}
			}

			minus.addEventListener("click", () => update(line.quantity - 1));
			plus.addEventListener("click", () => update(line.quantity + 1));
			remove.addEventListener("click", drop);
		}

		const price = el("p", "cart-line__price", formatMoney(cart.currency, line.available ? line.lineTotal : 0));
		row.append(media, body, price);
		return row;
	}

	function renderCartResponse(cart, list, errorEl) {
		setBadge(cart.itemCount);
		renderSummary(cart);
		setCheckoutState(cart);
		const summary = document.getElementById("cart-summary");
		if (summary) summary.hidden = cart.lines.length === 0;
		list.textContent = "";
		if (!cart.lines.length) {
			list.append(emptyCartPanel());
			return;
		}
		for (const line of cart.lines) list.append(buildCartLine(cart, line, list, errorEl));
	}

	// ---------- checkout page ----------

	async function initCheckoutPage() {
		const page = document.querySelector("[data-checkout-page]");
		if (!page) return;
		const form = document.getElementById("checkout-form");
		const summary = document.getElementById("checkout-lines");
		const errorEl = document.getElementById("checkout-error");
		const submit = document.getElementById("place-order");
		if (!form) return;

		// Prefill email/name for logged-in buyers (best-effort): the auth
		// hint gates the probe so anonymous loads issue zero profile
		// requests. A failed fetch leaves the fields blank and the guest
		// flow intact. The session travels in the HttpOnly cookie, so no
		// token is read.
		if (hasAuthHint()) {
			try {
				const res = await fetch("/api/profile/", {
					headers: { Accept: "application/json" },
					credentials: "same-origin",
				});
				if (res.ok) {
					const payload = await res.json().catch(() => null);
					const profile = payload && payload.data && payload.data.profile;
					if (profile) {
						const emailInput = document.getElementById("checkout-email");
						const nameInput = document.getElementById("checkout-name");
						if (emailInput && !emailInput.value && profile.email) emailInput.value = profile.email;
						if (nameInput && !nameInput.value && profile.name) nameInput.value = profile.name;
					}
				}
			} catch (_) {
				/* prefill is best-effort: the guest flow stays intact */
			}
		}

		let cart = null;
		if (readToken()) {
			try {
				cart = await getCart();
			} catch (error) {
				if (error.status === 401) writeToken("");
			}
		}

		const unavailable = Boolean(cart && cart.lines.some((line) => !line.available));
		if (!cart || !cart.lines.length || !cart.itemCount || unavailable) {
			const message = unavailable ? copy.unavailableCheckout : copy.emptyCheckout;
			if (summary) {
				summary.textContent = "";
				summary.append(el("p", "empty", message));
			}
			if (submit) submit.disabled = true;
			showError(errorEl, message);
			return;
		}

		setBadge(cart.itemCount);
		renderSummary(cart);
		if (summary) {
			summary.textContent = "";
			for (const line of cart.lines) {
				if (!line.available) continue;
				const row = el("div", "summary-line");
				const left = el("div");
				left.append(
					el("div", "summary-line__title", (line.title || "Item") + " × " + line.quantity),
					el("div", "summary-line__meta", line.sku || ""),
				);
				row.append(left, el("div", "", formatMoney(cart.currency, line.lineTotal)));
				summary.append(row);
			}
		}

		form.addEventListener("submit", async (event) => {
			event.preventDefault();
			clearError(errorEl);
			if (submit) submit.disabled = true;
			const data = new FormData(form);
			const email = String(data.get("email") || "").trim();
			const name = String(data.get("name") || "").trim();
			const phone = String(data.get("phone") || "").trim();
			const shipTo = {
				street: String(data.get("street") || "").trim(),
				city: String(data.get("city") || "").trim(),
				province: String(data.get("province") || "").trim(),
				postalCode: String(data.get("postalCode") || "").trim(),
			};
			const consent = document.getElementById("checkout-consent");
			if (consent && !consent.checked) {
				showError(errorEl, copy.consentRequired);
				if (submit) submit.disabled = false;
				return;
			}
			try {
				const order = await request("/checkout", {
					method: "POST",
					cart: true,
					body: { token: readToken(), email, name, phone, shipTo, consentAccepted: consent ? consent.checked : false },
				});
				writeToken("");
				setBadge(0);
				// The tracking token is the order's guest key: it arrives
				// exactly once, in this response and the confirmation
				// email. Keep both.
				writeLastOrder({ orderNumber: order.orderNumber, email, token: order.trackingToken || "" });
				window.location.href = "/orders/track?order=" + encodeURIComponent(order.orderNumber)
					+ (order.trackingToken ? "&t=" + encodeURIComponent(order.trackingToken) : "");
			} catch (error) {
				showError(errorEl, error.message);
				if (submit) submit.disabled = false;
			}
		});
	}

	// ---------- order tracking + payment ----------

	function renderOrder(order) {
		const root = document.getElementById("order-root");
		if (!root) return;
		root.textContent = "";
		root.hidden = false;

		const card = el("section", "order-card");
		const head = el("header", "order-card__head");
		const heading = el("div");
		heading.append(el("p", "order-card__number", copy.orderNumber + " " + order.orderNumber));
		heading.append(el("p", "order-card__date", copy.orderDate + " " + formatDate(order.createdAt)));
		const badge = el("span", "status-badge is-" + order.status, statusLabels[order.status] || order.status);
		head.append(heading, badge);
		card.append(head);

		if (statusOrder[order.status] !== undefined) {
			const line = el("div", "order-timeline");
			const reached = statusOrder[order.status];
			for (const step of timeline) {
				const node = el("div", "order-timeline__step", step.label);
				if (statusOrder[step.status] <= reached) node.classList.add("is-done");
				line.append(node);
			}
			card.append(line);
		}

		const lines = el("div", "order-lines");
		for (const item of order.items || []) {
			const row = el("div", "order-line");
			const left = el("div");
			left.append(el("div", "", (item.title || "Item") + " × " + item.quantity));
			const meta = [item.sku, formatMoney(order.currency, item.unitPrice) + " / " + (isID ? "pcs" : "pc")].filter(Boolean).join(" · ");
			left.append(el("div", "order-line__meta", meta));
			row.append(left, el("div", "", formatMoney(order.currency, item.lineTotal)));
			lines.append(row);
		}
		card.append(lines);

		const totals = el("dl", "order-totals");
		const subtotal = el("div");
		subtotal.append(el("dt", "", copy.subtotal), el("dd", "", formatMoney(order.currency, order.subtotal)));
		const shipping = el("div");
		shipping.append(el("dt", "", copy.shipping), el("dd", "", formatMoney(order.currency, order.shippingMinor)));
		const total = el("div", "order-totals__total");
		total.append(el("dt", "", copy.total), el("dd", "", formatMoney(order.currency, order.totalMinor)));
		totals.append(subtotal, shipping, total);
		card.append(totals);

		const address = el("div", "order-address");
		address.append(el("strong", "", copy.address));
		const linesAddr = [order.shipTo && order.shipTo.street, order.shipTo && order.shipTo.city, order.shipTo && order.shipTo.province, order.shipTo && order.shipTo.postalCode]
			.filter(Boolean)
			.join(", ");
		address.append(el("p", "", linesAddr));
		card.append(address);

		root.append(card);
	}

	function renderPayment(order, email, token) {
		const root = document.getElementById("payment-root");
		const errorEl = document.getElementById("track-error");
		if (!root) return;
		root.textContent = "";
		if (order.status !== "pending") {
			root.hidden = true;
			return;
		}
		root.hidden = false;

		const card = el("section", "payment-card");
		card.append(el("h2", "", isID ? "Selesaikan pembayaran" : "Complete your payment"));
		card.append(
			el(
				"p",
				"",
				isID
					? "Pilih metode pembayaran. Instruksi lengkap ada di email pesananmu."
					: "Choose a payment method. Full instructions are in your order email.",
			),
		);
		const amount = el("p", "payment-amount");
		amount.append(el("span", "", copy.total), el("strong", "", formatMoney(order.currency, order.totalMinor)));
		card.append(amount);

		if (order.manualPayee && order.manualPayee.bank) {
			const payee = el("div", "payment-payee");
			payee.append(el("h3", "", isID ? "Cara membayar (transfer manual)" : "How to pay (manual transfer)"));
			const list = el("dl", "payment-payee__list");
			const bankRow = el("div");
			bankRow.append(el("dt", "", "Bank"), el("dd", "", order.manualPayee.bank));
			const accRow = el("div");
			accRow.append(el("dt", "", isID ? "Rekening" : "Account"), el("dd", "", order.manualPayee.account));
			const nameRow = el("div");
			nameRow.append(el("dt", "", isID ? "Nama" : "Name"), el("dd", "", order.manualPayee.name));
			list.append(bankRow, accRow, nameRow);
			payee.append(list);
			card.append(payee);
		}

		const methods = el("div", "payment-methods");
		const formRoot = el("div");
		card.append(methods, formRoot);
		root.append(card);

		let selected = "";

		function renderForm(method) {
			selected = method;
			formRoot.textContent = "";
			for (const chip of methods.querySelectorAll(".payment-method")) {
				const active = chip.dataset.method === method;
				chip.classList.toggle("is-active", active);
				chip.setAttribute("aria-pressed", active ? "true" : "false");
			}
			if (method === "midtrans") {
				const button = el("button", "btn", copy.payNow);
				button.type = "button";
				button.addEventListener("click", async () => {
					button.disabled = true;
					try {
						const session = await request("/orders/" + encodeURIComponent(order.orderNumber) + "/payments/session", {
							method: "POST",
							body: { email, token },
						});
						window.location.href = session.redirectUrl;
					} catch (error) {
						button.disabled = false;
						showError(errorEl, error.message);
					}
				});
				formRoot.append(button);
				return;
			}
			const form = el("form", "payment-form");
			const field = el("div", "field");
			const label = el("label", "", copy.reference);
			const input = el("input", "form-control");
			input.id = "payment-reference";
			input.type = "text";
			input.required = true;
			input.placeholder = copy.referenceHint;
			input.autocomplete = "off";
			label.htmlFor = input.id;
			field.append(label, input);
			const submit = el("button", "btn", copy.submitProof);
			submit.type = "submit";
			form.append(field, submit);
			form.addEventListener("submit", async (event) => {
				event.preventDefault();
				clearError(errorEl);
				submit.disabled = true;
				try {
					await request("/orders/" + encodeURIComponent(order.orderNumber) + "/payments", {
						method: "POST",
						body: { email, token, method: "manual", reference: input.value.trim() },
					});
					form.textContent = "";
					const success = el("p", "alert alert--success", copy.submitted);
					form.append(success);
				} catch (error) {
					showError(errorEl, error.message);
					submit.disabled = false;
				}
			});
			formRoot.append(form);
		}

		(async () => {
			let available = [];
			try {
				const data = await request("/payments/methods");
				available = (data && data.methods) || [];
			} catch (_) {
				available = [];
			}
			if (!available.length) {
				methods.remove();
				card.append(el("p", "muted", copy.noPayment));
				return;
			}
			for (const method of available) {
				const label = method === "manual" ? copy.methodManual : method === "midtrans" ? copy.methodMidtrans : method;
				const chip = el("button", "payment-method", label);
				chip.type = "button";
				chip.dataset.method = method;
				chip.setAttribute("aria-pressed", "false");
				chip.addEventListener("click", () => renderForm(method));
				methods.append(chip);
			}
			renderForm(available[0]);
		})();
	}

	async function initTrackPage() {
		const page = document.querySelector("[data-track-page]");
		if (!page) return;
		const form = document.getElementById("track-form");
		const orderInput = document.getElementById("track-order");
		const emailInput = document.getElementById("track-email");
		const errorEl = document.getElementById("track-error");
		const orderRoot = document.getElementById("order-root");
		const paymentRoot = document.getElementById("payment-root");
		if (!form) return;

		const tokenInput = document.getElementById("track-token");
		const params = new URLSearchParams(window.location.search);
		const last = readLastOrder();
		// Gateway finish redirects can arrive double-encoded: Midtrans
		// re-encodes the callback URL when appending its own params, so an
		// email decoded once still carries escapes (%40). Decode one extra
		// layer when escapes remain; a malformed escape keeps the original.
		function decodeOnceParam(value) {
			if (!value || value.indexOf("%") === -1) return value;
			try {
				return decodeURIComponent(value);
			} catch (_) {
				return value;
			}
		}
		function trackParam(name) {
			return decodeOnceParam(params.get(name));
		}
		if (orderInput && trackParam("order")) orderInput.value = trackParam("order");
		if (tokenInput && trackParam("t")) tokenInput.value = trackParam("t");
		// The gateway finish redirect carries order + email + token, so a
		// buyer landing back here auto-loads their paid order below.
		if (emailInput && trackParam("email")) emailInput.value = trackParam("email");
		if (emailInput && last && last.email && !emailInput.value) emailInput.value = last.email;
		if (tokenInput && !tokenInput.value && last && last.token) tokenInput.value = last.token;

		async function load(orderNumber, email, token) {
			clearError(errorEl);
			if (orderRoot) orderRoot.hidden = true;
			if (paymentRoot) paymentRoot.hidden = true;
			try {
				const order = await request(
					"/orders/" + encodeURIComponent(orderNumber)
					+ "?email=" + encodeURIComponent(email)
					+ "&token=" + encodeURIComponent(token),
				);
				writeLastOrder({ orderNumber: order.orderNumber, email, token });
				renderOrder(order);
				renderPayment(order, email, token);
				const url = new URL(window.location.href);
				url.searchParams.set("order", order.orderNumber);
				url.searchParams.set("t", token);
				window.history.replaceState({}, "", url);
			} catch (error) {
				showError(errorEl, error && error.status === 404 ? copy.orderLookup : copy.genericError);
			}
		}

		form.addEventListener("submit", (event) => {
			event.preventDefault();
			const orderNumber = String(orderInput.value || "").trim();
			const email = String(emailInput.value || "").trim();
			const token = String((tokenInput && tokenInput.value) || "").trim();
			if (!orderNumber || !email || !token) return;
			load(orderNumber, email, token);
		});

		if (orderInput && orderInput.value && emailInput && emailInput.value) {
			const token = String((tokenInput && tokenInput.value) || "").trim();
			if (token) load(orderInput.value.trim(), emailInput.value.trim(), token);
		}
	}

	// ---------- boot ----------

	function boot() {
		initCookieBanner();
		refreshBadge();
		// Currency config loads concurrently: price hydration must not wait
		// for it. initCurrencyToggle refreshes rendered prices once the
		// config (or its failure) resolves.
		initCurrencyToggle().finally(() => {
			refreshDisplayPrices();
		});
		initLanguageSelect();
		initProductPage();
		hydrateCardPrices();
		initCartPage();
		initCheckoutPage();
		initTrackPage();
	}

	if (document.readyState === "loading") {
		document.addEventListener("DOMContentLoaded", boot);
	} else {
		boot();
	}
})();
