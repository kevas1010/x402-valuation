from publisher.tickers import DEFAULT_TICKERS, parse_ticker_list


def test_default_universe_has_tested_names_and_twenty():
    assert "AEHR" in DEFAULT_TICKERS
    assert "TER" in DEFAULT_TICKERS
    assert "MSFT" in DEFAULT_TICKERS
    assert 10 <= len(DEFAULT_TICKERS) <= 20


def test_parse_ticker_list():
    assert parse_ticker_list("aehr, ter") == ["AEHR", "TER"]
    assert parse_ticker_list(None) == DEFAULT_TICKERS
