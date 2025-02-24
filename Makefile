# Docker build targets use an optional "TAG" environment
# variable can be set to use custom tag name. For example:
#   TAG=my-registry.example.com/keystore:dev make keystore
XDRS = xdr/Stellar-SCP.x \
xdr/Stellar-ledger-entries.x \
xdr/Stellar-ledger.x \
xdr/Stellar-overlay.x \
xdr/Stellar-transaction.x \
xdr/Stellar-types.x

XDRGEN_COMMIT=9fccddf09dc8888d3b48be61404f0ab60149619e

.PHONY: xdr xdr-clean xdr-update

keystore:
	$(MAKE) -C services/keystore/ docker-build

ticker:
	$(MAKE) -C services/ticker/ docker-build

friendbot:
	$(MAKE) -C services/friendbot/ docker-build

webauth:
	$(MAKE) -C exp/services/webauth/ docker-build

recoverysigner:
	$(MAKE) -C exp/services/recoverysigner/ docker-build

regulated-assets-approval-server:
	$(MAKE) -C services/regulated-assets-approval-server/ docker-build

gxdr/xdr_generated.go: $(XDRS)
	go run github.com/xdrpp/goxdr/cmd/goxdr -p gxdr -enum-comments -o $@ $(XDRS)
	gofmt -s -w $@

xdr/%.x:
	curl -Lsf -o $@ https://raw.githubusercontent.com/stellar/stellar-core/master/src/protocol-curr/$@

xdr/xdr_generated.go: $(XDRS)
	docker run -it --rm -v $$PWD:/wd -w /wd ruby /bin/bash -c '\
		gem install specific_install -v 0.3.7 && \
		gem specific_install https://github.com/stellar/xdrgen.git -b $(XDRGEN_COMMIT) && \
		xdrgen \
			--language go \
			--namespace xdr \
			--output xdr/ \
			$(XDRS)'
	gofmt -s -w $@

xdr: gxdr/xdr_generated.go xdr/xdr_generated.go

xdr-clean:
	rm xdr/*.x || true

xdr-update: xdr-clean xdr
