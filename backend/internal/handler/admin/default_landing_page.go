package admin

// DefaultLandingPageHTML 默认落地页 HTML 模板
var DefaultLandingPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>{{.AppName}}</title>
<style>
*,*::before,*::after{box-sizing:border-box;margin:0;padding:0}
:root{
  --primary:{{.PrimaryColor}};
  --bg:#050507;--surface:#0c0c14;--surface2:#141420;--surface3:#1c1c2e;
  --text:#f0f0f5;--text2:#8888a0;--text3:#55556a;
  --border:rgba(255,255,255,.06);--border2:rgba(255,255,255,.1);
  --glow:hsla(var(--primary),.12);--glow2:hsla(var(--primary),.25);
  --radius:16px;
}
html{scroll-behavior:smooth}
body{font-family:system-ui,-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;background:var(--bg);color:var(--text);line-height:1.65;overflow-x:hidden;-webkit-font-smoothing:antialiased}
a{color:inherit;text-decoration:none}
img{display:block;max-width:100%}

@keyframes float{0%,100%{transform:translateY(0)}50%{transform:translateY(-20px)}}
@keyframes pulse-glow{0%,100%{opacity:.4}50%{opacity:.8}}
@keyframes spin{to{transform:rotate(360deg)}}
@keyframes fade-up{from{opacity:0;transform:translateY(30px)}to{opacity:1;transform:translateY(0)}}
@keyframes marquee{from{transform:translateX(0)}to{transform:translateX(-50%)}}
.fade-in{opacity:0;transform:translateY(30px);transition:opacity .7s ease,transform .7s ease}
.fade-in.visible{opacity:1;transform:translateY(0)}

.orb{position:absolute;border-radius:50%;filter:blur(80px);pointer-events:none;will-change:transform}
.orb-1{width:500px;height:500px;background:hsla(var(--primary),.08);top:-150px;left:-100px;animation:float 8s ease-in-out infinite}
.orb-2{width:400px;height:400px;background:hsla(280,80%,60%,.06);bottom:-100px;right:-80px;animation:float 10s ease-in-out infinite 2s}
.orb-3{width:300px;height:300px;background:hsla(var(--primary),.06);top:40%;left:60%;animation:float 12s ease-in-out infinite 4s}

.nav{position:fixed;top:0;left:0;right:0;z-index:100;transition:all .3s}
.nav.scrolled{background:rgba(5,5,7,.85);backdrop-filter:blur(20px) saturate(1.8);border-bottom:1px solid var(--border)}
.nav-inner{max-width:1200px;margin:0 auto;padding:0 24px;height:68px;display:flex;align-items:center;justify-content:space-between}
.nav-brand{display:flex;align-items:center;gap:10px;font-size:1.2rem;font-weight:700;letter-spacing:-.02em}
.nav-brand img{height:30px;width:auto;border-radius:8px}
.nav-links{display:flex;align-items:center;gap:6px}
.nav-links a{padding:8px 16px;border-radius:10px;font-size:.85rem;font-weight:500;transition:all .25s;color:var(--text2)}
.nav-links a:hover{color:var(--text);background:rgba(255,255,255,.05)}
.nav-links .btn-cta{background:hsl(var(--primary));color:#fff;font-weight:600}
.nav-links .btn-cta:hover{opacity:.9;transform:translateY(-1px);box-shadow:0 4px 20px var(--glow2)}

.hero{min-height:100vh;display:flex;align-items:center;justify-content:center;text-align:center;padding:100px 24px 80px;position:relative;overflow:hidden}
.hero-grid{position:absolute;inset:0;background-image:linear-gradient(rgba(255,255,255,.03) 1px,transparent 1px),linear-gradient(90deg,rgba(255,255,255,.03) 1px,transparent 1px);background-size:60px 60px;mask-image:radial-gradient(ellipse 60% 50% at 50% 50%,black 20%,transparent 100%)}
.hero-content{position:relative;max-width:800px;animation:fade-up .8s ease both}
.hero-pill{display:inline-flex;align-items:center;gap:8px;padding:7px 18px 7px 10px;border-radius:999px;background:var(--surface2);border:1px solid var(--border2);font-size:.8rem;color:var(--text2);margin-bottom:32px;transition:all .3s}
.hero-pill:hover{border-color:hsla(var(--primary),.3);background:var(--surface3)}
.hero-pill .dot{width:8px;height:8px;border-radius:50%;background:hsl(var(--primary));animation:pulse-glow 2s infinite}
.hero-pill .arrow{margin-left:4px;opacity:.5;transition:all .2s}
.hero-pill:hover .arrow{opacity:1;transform:translateX(2px)}
.hero h1{font-size:clamp(2.8rem,7vw,4.5rem);font-weight:800;line-height:1.08;letter-spacing:-.03em;margin-bottom:24px}
.hero h1 .gradient{background:linear-gradient(135deg,hsl(var(--primary)),#a78bfa);-webkit-background-clip:text;-webkit-text-fill-color:transparent;background-clip:text}
.hero-sub{font-size:1.15rem;color:var(--text2);max-width:520px;margin:0 auto;line-height:1.7}
.btn-lg{padding:14px 32px;border-radius:12px;font-size:.95rem;font-weight:600;border:none;cursor:pointer;transition:all .25s;display:inline-flex;align-items:center;gap:8px}
.btn-fill{background:hsl(var(--primary));color:#fff}
.btn-fill:hover{transform:translateY(-2px);box-shadow:0 8px 32px var(--glow2)}
.hero-stats{display:flex;justify-content:center;gap:48px;margin-top:40px;padding-top:32px;border-top:1px solid var(--border)}
.hero-stat .num{font-size:1.8rem;font-weight:800;letter-spacing:-.02em;background:linear-gradient(135deg,var(--text),var(--text2));-webkit-background-clip:text;-webkit-text-fill-color:transparent;background-clip:text}
.hero-stat .label{font-size:.75rem;color:var(--text3);margin-top:4px;text-transform:uppercase;letter-spacing:.08em}

@keyframes bounce-down{0%,100%{transform:translateY(0)}50%{transform:translateY(6px)}}
.scroll-hint{position:absolute;bottom:32px;left:50%;transform:translateX(-50%);display:flex;flex-direction:column;align-items:center;gap:6px;color:var(--text3);font-size:.7rem;letter-spacing:.08em;text-transform:uppercase;opacity:.6;transition:opacity .3s}
.scroll-hint svg{width:20px;height:20px;animation:bounce-down 2s ease-in-out infinite}

.section{padding:80px 24px;position:relative}
.section-inner{max-width:1200px;margin:0 auto}
.section-label{display:inline-block;font-size:.72rem;font-weight:600;text-transform:uppercase;letter-spacing:.1em;color:hsl(var(--primary));margin-bottom:12px;padding:5px 14px;border-radius:8px;background:hsla(var(--primary),.08);border:1px solid hsla(var(--primary),.15)}
.section-header{text-align:center;margin-bottom:56px}
.section-header h2{font-size:clamp(1.8rem,4vw,2.5rem);font-weight:800;letter-spacing:-.02em;margin-bottom:14px}
.section-header p{color:var(--text2);font-size:1.05rem;max-width:500px;margin:0 auto}

.products-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(270px,1fr));gap:20px}
.product-card{background:var(--surface);border:1px solid var(--border);border-radius:var(--radius);overflow:hidden;transition:all .35s ease;cursor:pointer;position:relative}
.product-card::after{content:'';position:absolute;inset:0;border-radius:var(--radius);opacity:0;transition:opacity .35s;background:linear-gradient(135deg,hsla(var(--primary),.08),transparent 60%);pointer-events:none}
.product-card:hover{transform:translateY(-6px);border-color:hsla(var(--primary),.2);box-shadow:0 20px 50px rgba(0,0,0,.5),0 0 40px var(--glow)}
.product-card:hover::after{opacity:1}
.product-img-wrap{position:relative;overflow:hidden}
.product-img{width:100%;aspect-ratio:4/3;object-fit:cover;background:var(--surface2);transition:transform .5s ease}
.product-card:hover .product-img{transform:scale(1.05)}
.product-info{padding:18px}
.product-info h3{font-size:.95rem;font-weight:600;margin-bottom:6px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.product-info .desc{font-size:.8rem;color:var(--text2);margin-bottom:12px;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden;min-height:2.5em;line-height:1.55}
.product-meta{display:flex;align-items:center;justify-content:space-between}
.product-price{font-size:1.1rem;font-weight:700;color:hsl(var(--primary))}
.product-badge{font-size:.7rem;padding:4px 10px;border-radius:8px;background:hsla(var(--primary),.1);color:hsl(var(--primary));font-weight:500}

.features-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:20px}
.feature-card{background:var(--surface);border:1px solid var(--border);border-radius:var(--radius);padding:32px;transition:all .35s;position:relative;overflow:hidden}
.feature-card::before{content:'';position:absolute;top:0;left:0;right:0;height:2px;background:linear-gradient(90deg,transparent,hsl(var(--primary)),transparent);opacity:0;transition:opacity .35s}
.feature-card:hover{border-color:var(--border2);transform:translateY(-4px)}
.feature-card:hover::before{opacity:1}
.feature-icon{width:48px;height:48px;border-radius:12px;background:hsla(var(--primary),.1);border:1px solid hsla(var(--primary),.15);display:flex;align-items:center;justify-content:center;margin-bottom:20px;font-size:1.3rem}
.feature-card h3{font-size:1.05rem;font-weight:600;margin-bottom:10px}
.feature-card p{font-size:.875rem;color:var(--text2);line-height:1.65}

.marquee-section{padding:50px 0;border-top:1px solid var(--border);border-bottom:1px solid var(--border);overflow:hidden}
.marquee-track{display:flex;gap:48px;animation:marquee 30s linear infinite;width:max-content}
.marquee-item{font-size:1.4rem;font-weight:700;color:var(--text3);white-space:nowrap;display:flex;align-items:center;gap:16px;letter-spacing:-.01em}
.marquee-dot{width:6px;height:6px;border-radius:50%;background:var(--text3);opacity:.4}

.cta{text-align:center;padding:80px 24px;position:relative;overflow:hidden}
.cta-bg{position:absolute;inset:0;background:radial-gradient(ellipse 80% 60% at 50% 100%,hsla(var(--primary),.1),transparent 70%)}
.cta-inner{position:relative;max-width:600px;margin:0 auto}
.cta h2{font-size:clamp(2rem,5vw,2.8rem);font-weight:800;letter-spacing:-.02em;margin-bottom:16px}
.cta p{color:var(--text2);font-size:1.05rem;margin-bottom:36px;line-height:1.7}

.footer{border-top:1px solid var(--border);padding:40px 24px}
.footer-inner{max-width:1200px;margin:0 auto;display:flex;align-items:center;justify-content:space-between}
.footer-copy{font-size:.8rem;color:var(--text3)}
.footer-links{display:flex;gap:20px}
.footer-links a{font-size:.8rem;color:var(--text3);transition:color .2s}
.footer-links a:hover{color:var(--text2)}

.loading{display:flex;flex-direction:column;align-items:center;gap:12px;padding:60px 0;grid-column:1/-1}
.spinner{width:28px;height:28px;border:2.5px solid var(--border);border-top-color:hsl(var(--primary));border-radius:50%;animation:spin .7s linear infinite}
.loading-text{font-size:.8rem;color:var(--text3)}

@media(max-width:768px){
  .nav-links a:not(.btn-cta){display:none}
  .hero{padding:100px 20px 70px}
  .hero h1{font-size:2.2rem}
  .hero-sub{font-size:1rem}
  .hero-stats{gap:24px;flex-wrap:wrap}
  .hero-stat .num{font-size:1.4rem}
  .scroll-hint{bottom:20px}
  .features-grid{grid-template-columns:1fr}
  .products-grid{grid-template-columns:repeat(auto-fill,minmax(220px,1fr));gap:14px}
  .section{padding:56px 20px}
  .cta{padding:60px 20px}
  .footer-inner{flex-direction:column;gap:16px;text-align:center}
}
</style>
</head>
<body>
`

var defaultLandingPageBody = `
<nav class="nav" id="nav">
  <div class="nav-inner">
    <a href="/" class="nav-brand">
      {{if .LogoURL}}<img src="{{.LogoURL}}" alt="{{.AppName}}">{{end}}
      {{.AppName}}
    </a>
    <div class="nav-links">
      <a href="/products">Products</a>
      <a href="/login">Sign In</a>
      <a href="/login" class="btn-cta">Get Started</a>
    </div>
  </div>
</nav>

<section class="hero">
  <div class="hero-grid"></div>
  <div class="orb orb-1"></div>
  <div class="orb orb-2"></div>
  <div class="hero-content">
    <a href="/products" class="hero-pill">
      <span class="dot"></span>
      Explore our collection
      <span class="arrow">&rarr;</span>
    </a>
    <h1><span class="gradient">Premium Products</span><br>Delivered With Care</h1>
    <p class="hero-sub">Curated selection, secure checkout, and reliable fulfillment. Everything you need in one place.</p>
    <div class="hero-stats" id="heroStats"></div>
  </div>
  <a href="#products" class="scroll-hint">
    <span>Scroll</span>
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M7 13l5 5 5-5"/><path d="M7 6l5 5 5-5"/></svg>
  </a>
</section>

<section class="section" id="products">
  <div class="section-inner fade-in">
    <div class="section-header">
      <span class="section-label">Shop</span>
      <h2>Recommended Products</h2>
      <p>Top picks selected just for you</p>
    </div>
    <div id="productGrid" class="products-grid">
      <div class="loading"><div class="spinner"></div><span class="loading-text">Loading products...</span></div>
    </div>
  </div>
</section>

<div class="marquee-section">
  <div class="marquee-track" id="marquee">
    <span class="marquee-item">Fast Delivery <span class="marquee-dot"></span></span>
    <span class="marquee-item">Secure Payments <span class="marquee-dot"></span></span>
    <span class="marquee-item">Quality Guaranteed <span class="marquee-dot"></span></span>
    <span class="marquee-item">24/7 Support <span class="marquee-dot"></span></span>
    <span class="marquee-item">Easy Returns <span class="marquee-dot"></span></span>
    <span class="marquee-item">Digital Delivery <span class="marquee-dot"></span></span>
    <span class="marquee-item">Fast Delivery <span class="marquee-dot"></span></span>
    <span class="marquee-item">Secure Payments <span class="marquee-dot"></span></span>
    <span class="marquee-item">Quality Guaranteed <span class="marquee-dot"></span></span>
    <span class="marquee-item">24/7 Support <span class="marquee-dot"></span></span>
    <span class="marquee-item">Easy Returns <span class="marquee-dot"></span></span>
    <span class="marquee-item">Digital Delivery <span class="marquee-dot"></span></span>
  </div>
</div>

<section class="section">
  <div class="section-inner fade-in">
    <div class="section-header">
      <span class="section-label">Why Us</span>
      <h2>Built for Reliability</h2>
      <p>A seamless experience from browsing to delivery</p>
    </div>
    <div class="features-grid">
      <div class="feature-card fade-in">
        <div class="feature-icon">&#9889;</div>
        <h3>Instant Digital Delivery</h3>
        <p>Digital products are delivered automatically right after payment confirmation. No waiting.</p>
      </div>
      <div class="feature-card fade-in">
        <div class="feature-icon">&#128737;</div>
        <h3>Secure Transactions</h3>
        <p>Enterprise-grade security protects every transaction. Multiple payment methods supported.</p>
      </div>
      <div class="feature-card fade-in">
        <div class="feature-icon">&#128172;</div>
        <h3>Dedicated Support</h3>
        <p>Built-in ticket system for quick issue resolution. We respond fast and solve problems.</p>
      </div>
    </div>
  </div>
</section>

<section class="cta">
  <div class="cta-bg"></div>
  <div class="orb orb-3"></div>
  <div class="cta-inner fade-in">
    <h2>Start Shopping Today</h2>
    <p>Join our community and discover products you will love. Quick signup, instant access.</p>
    <a href="/login" class="btn-lg btn-fill">Create Free Account</a>
  </div>
</section>

<footer class="footer">
  <div class="footer-inner">
    <span class="footer-copy">&copy; {{.Year}} {{.AppName}}. All rights reserved.</span>
    <div class="footer-links">
      <a href="/products">Products</a>
      <a href="/login">Account</a>
    </div>
  </div>
</footer>

<script>
(function(){
  // Nav scroll effect
  var nav=document.getElementById('nav');
  window.addEventListener('scroll',function(){nav.classList.toggle('scrolled',window.scrollY>30)});

  // Fade-in on scroll
  var obs=new IntersectionObserver(function(entries){entries.forEach(function(e){if(e.isIntersecting){e.target.classList.add('visible');obs.unobserve(e.target)}})},{threshold:.15});
  document.querySelectorAll('.fade-in').forEach(function(el){obs.observe(el)});

  // Load products
  var grid=document.getElementById('productGrid');
  var stats=document.getElementById('heroStats');
  var currency='{{.Currency}}';

  fetch('/api/user/products/recommended?limit=8')
    .then(function(r){return r.json()})
    .then(function(res){
      var items=res.data&&res.data.products||[];
      if(!items.length){
        grid.innerHTML='<div class="loading" style="padding:40px 0"><p style="color:var(--text3)">No products available yet.</p></div>';
        return;
      }
      // Stats
      stats.innerHTML='<div class="hero-stat"><div class="num">'+items.length+'+</div><div class="label">Products</div></div><div class="hero-stat"><div class="num">24/7</div><div class="label">Support</div></div><div class="hero-stat"><div class="num">100%</div><div class="label">Secure</div></div>';

      var html='';
      items.forEach(function(p){
        var img=p.images&&p.images.length?p.images[0]:'';
        var imgTag=img?'<div class="product-img-wrap"><img class="product-img" src="'+img+'" alt="'+p.name+'" loading="lazy"></div>':'<div class="product-img-wrap"><div class="product-img"></div></div>';
        html+='<a href="/products/'+p.id+'" class="product-card fade-in">'+imgTag+'<div class="product-info"><h3>'+p.name+'</h3><div class="desc">'+(p.short_description||p.description||'')+'</div><div class="product-meta"><span class="product-price">'+currency+' '+Number(p.price).toFixed(2)+'</span>'+(p.category?'<span class="product-badge">'+p.category+'</span>':'')+'</div></div></a>';
      });
      grid.innerHTML=html;
      // Observe new cards
      grid.querySelectorAll('.fade-in').forEach(function(el){obs.observe(el)});
    })
    .catch(function(){grid.innerHTML='<div class="loading" style="padding:40px 0"><p style="color:var(--text3)">Failed to load products.</p></div>'});
})();
</script>
</body>
</html>`

func init() {
	DefaultLandingPageHTML = DefaultLandingPageHTML + defaultLandingPageBody
}
