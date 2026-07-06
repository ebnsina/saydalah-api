#!/usr/bin/env python3
"""
Seed the Saydalah API with a generous, realistic data set for testing the full
system through its real endpoints (so FEFO dispensing, stock batches, and the
movement ledger are all exercised).

Usage:
    API_BASE=http://localhost:8080/api/v1 \
    ADMIN_EMAIL=admin@saydalah.test ADMIN_PASSWORD=supersecret123 \
    python3 scripts/seed.py

Idempotency: this appends data on every run (it does not clear existing rows).
Point it at a fresh database for a clean set.
"""

import json
import os
import random
import time
import urllib.error
import urllib.request
from datetime import date, timedelta

API_BASE = os.environ.get("API_BASE", "http://localhost:8080/api/v1").rstrip("/")
ADMIN_EMAIL = os.environ.get("ADMIN_EMAIL", "admin@saydalah.test")
ADMIN_PASSWORD = os.environ.get("ADMIN_PASSWORD", "supersecret123")

random.seed(42)  # deterministic runs

_token = None


def req(method, path, body=None, token=None):
    data = json.dumps(body).encode() if body is not None else None
    r = urllib.request.Request(API_BASE + path, data=data, method=method)
    r.add_header("Content-Type", "application/json")
    if token:
        r.add_header("Authorization", "Bearer " + token)
    for attempt in range(6):
        try:
            with urllib.request.urlopen(r) as resp:
                raw = resp.read()
                return json.loads(raw) if raw else None
        except urllib.error.HTTPError as e:
            if e.code == 429:  # rate limited — back off
                time.sleep(0.5 * (attempt + 1))
                continue
            msg = e.read().decode()
            raise RuntimeError(f"{method} {path} -> {e.code}: {msg}") from None
    raise RuntimeError(f"{method} {path} -> gave up after retries")


def login():
    global _token
    res = req("POST", "/auth/login", {"email": ADMIN_EMAIL, "password": ADMIN_PASSWORD})
    _token = res["access_token"]


def api(method, path, body=None):
    return req(method, path, body, token=_token)


# --- reference data ----------------------------------------------------------

BRANCHES = [
    {"name": "Saydalah Downtown", "address": "12 Market St", "phone": "+201000000001"},
    {"name": "Saydalah Uptown", "address": "48 Nile Ave", "phone": "+201000000002"},
    {"name": "Saydalah Riverside", "address": "3 Corniche Rd", "phone": "+201000000003"},
]

SUPPLIERS = [
    {"name": "MediSupply Co", "contact": "Layla Hassan", "phone": "+201110000001", "email": "sales@medisupply.test"},
    {"name": "PharmaDirect", "contact": "Omar Fahmy", "phone": "+201110000002", "email": "orders@pharmadirect.test"},
    {"name": "Global Drug Traders", "contact": "Nadia Saleh", "phone": "+201110000003", "email": "gdt@traders.test"},
    {"name": "Cairo Wholesale Pharma", "contact": "Karim Adel", "phone": "+201110000004", "email": "hello@cwp.test"},
    {"name": "Delta Medical", "contact": "Sara Mostafa", "phone": "+201110000005", "email": "supply@delta.test"},
]

FORMS = ["tablet", "capsule", "syrup", "injection", "cream", "drops", "inhaler"]
CATEGORIES = ["Analgesic", "Antibiotic", "Antihistamine", "Antacid", "Vitamin",
              "Antihypertensive", "Antidiabetic", "Dermatological", "Respiratory"]

# 40 realistic products (generic name, category)
PRODUCTS = [
    ("Paracetamol", "Analgesic"), ("Ibuprofen", "Analgesic"), ("Aspirin", "Analgesic"),
    ("Diclofenac", "Analgesic"), ("Naproxen", "Analgesic"),
    ("Amoxicillin", "Antibiotic"), ("Azithromycin", "Antibiotic"), ("Ciprofloxacin", "Antibiotic"),
    ("Doxycycline", "Antibiotic"), ("Cephalexin", "Antibiotic"), ("Metronidazole", "Antibiotic"),
    ("Cetirizine", "Antihistamine"), ("Loratadine", "Antihistamine"), ("Chlorpheniramine", "Antihistamine"),
    ("Fexofenadine", "Antihistamine"),
    ("Omeprazole", "Antacid"), ("Ranitidine", "Antacid"), ("Esomeprazole", "Antacid"),
    ("Pantoprazole", "Antacid"),
    ("Vitamin C", "Vitamin"), ("Vitamin D3", "Vitamin"), ("Vitamin B Complex", "Vitamin"),
    ("Multivitamin", "Vitamin"), ("Folic Acid", "Vitamin"), ("Iron + Folic", "Vitamin"),
    ("Amlodipine", "Antihypertensive"), ("Lisinopril", "Antihypertensive"), ("Losartan", "Antihypertensive"),
    ("Bisoprolol", "Antihypertensive"), ("Hydrochlorothiazide", "Antihypertensive"),
    ("Metformin", "Antidiabetic"), ("Gliclazide", "Antidiabetic"), ("Glimepiride", "Antidiabetic"),
    ("Insulin Glargine", "Antidiabetic"),
    ("Hydrocortisone", "Dermatological"), ("Clotrimazole", "Dermatological"), ("Betamethasone", "Dermatological"),
    ("Salbutamol", "Respiratory"), ("Budesonide", "Respiratory"), ("Montelukast", "Respiratory"),
]

STRENGTHS = ["250mg", "500mg", "5mg", "10mg", "20mg", "100mg", "125mg/5ml", "1000mg"]

CUSTOMERS = [
    "Ahmed Ali", "Mona Youssef", "Hassan Ibrahim", "Fatma Nabil", "Youssef Kamal",
    "Aya Farouk", "Tarek Sami", "Rania Gamal", "Khaled Ashraf", "Dina Magdy",
    "Mahmoud Reda", "Salma Tarek", "Amir Hosny", "Nour Hesham", "Bassem Wael",
]


def main():
    print(f"Seeding {API_BASE} …")
    login()

    branches = [api("POST", "/branches", b) for b in BRANCHES]
    print(f"  {len(branches)} branches")

    suppliers = [api("POST", "/suppliers", s) for s in SUPPLIERS]
    print(f"  {len(suppliers)} suppliers")

    products = []
    for i, (name, category) in enumerate(PRODUCTS):
        p = api("POST", "/products", {
            "name": name,
            "generic_name": name,
            "form": random.choice(FORMS),
            "strength": random.choice(STRENGTHS),
            "barcode": f"629{i:07d}",
            "category": category,
            "unit": "unit",
            "reorder_level": random.choice([20, 30, 50, 100]),
        })
        products.append(p)
    print(f"  {len(products)} products")

    # Staff per branch (pharmacist + cashier).
    staff = 0
    for b in branches:
        slug = b["name"].split()[-1].lower()
        for role in ("pharmacist", "cashier", "manager"):
            api("POST", "/users", {
                "email": f"{role}.{slug}@saydalah.test",
                "password": "password123",
                "full_name": f"{role.title()} {b['name'].split()[-1]}",
                "role": role,
                "branch_id": b["id"],
            })
            staff += 1
    print(f"  {staff} staff users")

    customers = [api("POST", "/customers", {"name": n, "phone": f"+2012{i:08d}"})
                 for i, n in enumerate(CUSTOMERS)]
    print(f"  {len(customers)} customers")

    today = date.today()
    total_pos, total_sales, total_rx = 0, 0, 0

    for b in branches:
        bid = b["id"]
        # Stock ~30 products in this branch via purchase orders + receipts.
        stocked = random.sample(products, 30)
        for prod in stocked:
            supplier = random.choice(suppliers)
            qty = random.choice([100, 150, 200, 300])
            cost = round(random.uniform(0.5, 8.0), 2)
            po = api("POST", "/purchase-orders", {
                "branch_id": bid,
                "supplier_id": supplier["id"],
                "reference": f"PO-{prod['name'][:4].upper()}-{random.randint(1000, 9999)}",
                "items": [{"product_id": prod["id"], "qty": qty, "unit_cost": cost}],
            })
            total_pos += 1
            # Receive two batches with different expiries (one near, one far).
            near = today + timedelta(days=random.randint(40, 120))
            far = today + timedelta(days=random.randint(300, 720))
            sale_price = round(cost * random.uniform(1.4, 2.5), 2)
            api("POST", f"/purchase-orders/{po['id']}/receive", {"lines": [
                {"product_id": prod["id"], "batch_no": f"B{near.year}{random.randint(100, 999)}",
                 "quantity": qty // 3, "cost_price": cost, "sale_price": sale_price,
                 "expiry_date": near.isoformat() + "T00:00:00Z"},
                {"product_id": prod["id"], "batch_no": f"B{far.year}{random.randint(100, 999)}",
                 "quantity": qty - qty // 3, "cost_price": cost, "sale_price": sale_price,
                 "expiry_date": far.isoformat() + "T00:00:00Z"},
            ]})

        # Ring up sales drawing from stocked products (modest quantities).
        for _ in range(60):
            line_products = random.sample(stocked, random.randint(1, 4))
            lines = [{"product_id": p["id"], "qty": random.randint(1, 5)} for p in line_products]
            body = {
                "branch_id": bid,
                "payment_method": random.choice(["cash", "card", "mobile"]),
                "lines": lines,
            }
            if random.random() < 0.4:
                body["customer_id"] = random.choice(customers)["id"]
            try:
                api("POST", "/sales", body)
                total_sales += 1
            except RuntimeError:
                pass  # occasional insufficient stock — skip

        # A few prescriptions (pharmacist/manager scope) with dispensing.
        for _ in range(8):
            cust = random.choice(customers)
            rx_products = random.sample(stocked, random.randint(1, 3))
            rx = api("POST", "/prescriptions", {
                "branch_id": bid,
                "customer_id": cust["id"],
                "doctor_name": random.choice(["Dr. Salah", "Dr. Amin", "Dr. Wahba", "Dr. Zaki"]),
                "items": [{"product_id": p["id"], "qty": random.randint(1, 3),
                           "dosage": "1 x daily"} for p in rx_products],
            })
            total_rx += 1
            if random.random() < 0.7:
                try:
                    api("POST", f"/prescriptions/{rx['id']}/dispense", {"payment_method": "cash"})
                except RuntimeError:
                    pass

    print(f"  {total_pos} purchase orders received")
    print(f"  {total_sales} sales")
    print(f"  {total_rx} prescriptions")
    print("Done.")


if __name__ == "__main__":
    main()
