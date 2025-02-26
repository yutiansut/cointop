package cointop

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

var fileperm = os.FileMode(0644)

type config struct {
	Shortcuts     map[string]interface{}   `toml:"shortcuts"`
	Favorites     map[string][]interface{} `toml:"favorites"`
	Portfolio     map[string]interface{}   `toml:"portfolio"`
	Currency      interface{}              `toml:"currency"`
	DefaultView   interface{}              `toml:"default_view"`
	CoinMarketCap map[string]interface{}   `toml:"coinmarketcap"`
	API           interface{}              `toml:"api"`
	Colorscheme   interface{}              `toml:"colorscheme"`
	RefreshRate   interface{}              `toml:"refresh_rate"`
}

func (ct *Cointop) setupConfig() error {
	ct.debuglog("setupConfig()")
	if err := ct.createConfigIfNotExists(); err != nil {
		return err
	}
	if err := ct.parseConfig(); err != nil {
		return err
	}
	if err := ct.loadShortcutsFromConfig(); err != nil {
		return err
	}
	if err := ct.loadFavoritesFromConfig(); err != nil {
		return err
	}
	if err := ct.loadPortfolioFromConfig(); err != nil {
		return err
	}
	if err := ct.loadCurrencyFromConfig(); err != nil {
		return err
	}
	if err := ct.loadDefaultViewFromConfig(); err != nil {
		return err
	}
	if err := ct.loadAPIKeysFromConfig(); err != nil {
		return err
	}
	if err := ct.loadAPIChoiceFromConfig(); err != nil {
		return err
	}
	if err := ct.loadColorschemeFromConfig(); err != nil {
		return err
	}
	if err := ct.loadRefreshRateFromConfig(); err != nil {
		return err
	}

	return nil
}

func (ct *Cointop) createConfigIfNotExists() error {
	ct.debuglog("createConfigIfNotExists()")
	err := ct.makeConfigDir()
	if err != nil {
		return err
	}

	// NOTE: legacy support for default path
	path := ct.configPath()
	oldConfigPath := NormalizePath(strings.Replace(path, "cointop/config.toml", "cointop/config", 1))
	if _, err := os.Stat(oldConfigPath); err == nil {
		path = oldConfigPath
		ct.configFilepath = oldConfigPath
		return nil
	}

	err = ct.makeConfigFile()
	if err != nil {
		return err
	}

	return nil
}

func (ct *Cointop) configDirPath() string {
	ct.debuglog("configDirPath()")
	path := NormalizePath(ct.configFilepath)
	separator := string(filepath.Separator)
	parts := strings.Split(path, separator)
	return strings.Join(parts[0:len(parts)-1], separator)
}

func (ct *Cointop) configPath() string {
	ct.debuglog("configPath()")
	return NormalizePath(ct.configFilepath)
}

func (ct *Cointop) makeConfigDir() error {
	ct.debuglog("makeConfigDir()")
	path := ct.configDirPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, os.ModePerm)
	}

	return nil
}

func (ct *Cointop) makeConfigFile() error {
	ct.debuglog("makeConfigFile()")
	path := ct.configPath()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fo, err := os.Create(path)
		if err != nil {
			return err
		}
		defer fo.Close()
		b, err := ct.configToToml()
		if err != nil {
			return err
		}
		if _, err := fo.Write(b); err != nil {
			return err
		}
	}
	return nil
}

func (ct *Cointop) saveConfig() error {
	ct.debuglog("saveConfig()")
	ct.saveMux.Lock()
	defer ct.saveMux.Unlock()
	path := ct.configPath()
	if _, err := os.Stat(path); err == nil {
		b, err := ct.configToToml()
		if err != nil {
			return err
		}
		err = ioutil.WriteFile(path, b, fileperm)
		if err != nil {
			return err
		}
	}
	return nil
}

func (ct *Cointop) parseConfig() error {
	ct.debuglog("parseConfig()")
	var conf config
	path := ct.configPath()
	if _, err := toml.DecodeFile(path, &conf); err != nil {
		return err
	}

	ct.config = conf
	return nil
}

func (ct *Cointop) configToToml() ([]byte, error) {
	ct.debuglog("configToToml()")
	shortcutsIfcs := map[string]interface{}{}
	for k, v := range ct.State.shortcutKeys {
		var i interface{} = v
		shortcutsIfcs[k] = i
	}

	var favorites []interface{}
	for k, ok := range ct.State.favorites {
		if ok {
			var i interface{} = k
			favorites = append(favorites, i)
		}
	}
	var favoritesBySymbol []interface{}
	favoritesIfcs := map[string][]interface{}{
		// DEPRECATED: favorites by 'symbol' is deprecated because of collisions. Kept for backward compatibility.
		"symbols": favoritesBySymbol,
		"names":   favorites,
	}

	portfolioIfc := map[string]interface{}{}
	for name := range ct.State.portfolio.Entries {
		entry, ok := ct.State.portfolio.Entries[name]
		if !ok || entry.Coin == "" {
			continue
		}
		var i interface{} = entry.Holdings
		portfolioIfc[entry.Coin] = i
	}

	var currencyIfc interface{} = ct.State.currencyConversion
	var defaultViewIfc interface{} = ct.State.defaultView
	var colorschemeIfc interface{} = ct.colorschemeName
	var refreshRateIfc interface{} = uint(ct.State.refreshRate.Seconds())

	cmcIfc := map[string]interface{}{
		"pro_api_key": ct.apiKeys.cmc,
	}
	var apiChoiceIfc interface{} = ct.apiChoice

	var inputs = &config{
		API:           apiChoiceIfc,
		Colorscheme:   colorschemeIfc,
		CoinMarketCap: cmcIfc,
		Currency:      currencyIfc,
		DefaultView:   defaultViewIfc,
		Favorites:     favoritesIfcs,
		RefreshRate:   refreshRateIfc,
		Shortcuts:     shortcutsIfcs,
		Portfolio:     portfolioIfc,
	}

	var b bytes.Buffer
	encoder := toml.NewEncoder(&b)
	err := encoder.Encode(inputs)
	if err != nil {
		return nil, err
	}

	return b.Bytes(), nil
}

func (ct *Cointop) loadShortcutsFromConfig() error {
	ct.debuglog("loadShortcutsFromConfig()")
	for k, ifc := range ct.config.Shortcuts {
		if v, ok := ifc.(string); ok {
			if !ct.ActionExists(v) {
				continue
			}
			if ct.State.shortcutKeys[k] == "" {
				continue
			}
			ct.State.shortcutKeys[k] = v
		}
	}
	return nil
}

func (ct *Cointop) loadCurrencyFromConfig() error {
	ct.debuglog("loadCurrencyFromConfig()")
	if currency, ok := ct.config.Currency.(string); ok {
		ct.State.currencyConversion = strings.ToUpper(currency)
	}
	return nil
}

func (ct *Cointop) loadDefaultViewFromConfig() error {
	ct.debuglog("loadDefaultViewFromConfig()")
	if defaultView, ok := ct.config.DefaultView.(string); ok {
		defaultView = strings.ToLower(defaultView)
		switch defaultView {
		case "portfolio":
			ct.State.portfolioVisible = true
		case "favorites":
			ct.State.filterByFavorites = true
		case "default":
			fallthrough
		default:
			ct.State.portfolioVisible = false
			ct.State.filterByFavorites = false
			defaultView = "default"
		}
		ct.State.defaultView = defaultView
	}
	return nil
}

func (ct *Cointop) loadAPIKeysFromConfig() error {
	ct.debuglog("loadAPIKeysFromConfig()")
	for key, value := range ct.config.CoinMarketCap {
		k := strings.TrimSpace(strings.ToLower(key))
		if k == "pro_api_key" {
			ct.apiKeys.cmc = value.(string)
		}
	}
	return nil
}

func (ct *Cointop) loadColorschemeFromConfig() error {
	ct.debuglog("loadColorschemeFromConfig()")
	if colorscheme, ok := ct.config.Colorscheme.(string); ok {
		ct.colorschemeName = colorscheme
	}

	return nil
}

func (ct *Cointop) loadRefreshRateFromConfig() error {
	ct.debuglog("loadRefreshRateFromConfig()")
	if refreshRate, ok := ct.config.RefreshRate.(int64); ok {
		ct.State.refreshRate = time.Duration(uint(refreshRate)) * time.Second
	}

	return nil
}

func (ct *Cointop) getColorschemeColors() (map[string]interface{}, error) {
	ct.debuglog("getColorschemeColors()")
	var colors map[string]interface{}
	if ct.colorschemeName == "" {
		ct.colorschemeName = defaultColorscheme
		if _, err := toml.Decode(DefaultColors, &colors); err != nil {
			return nil, err
		}
	} else {
		path := NormalizePath(fmt.Sprintf("~/.cointop/colors/%s.toml", ct.colorschemeName))
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// NOTE: case for when cointop is set as the theme but the colorscheme file doesn't exist
			if ct.colorschemeName == "cointop" {
				if _, err := toml.Decode(DefaultColors, &colors); err != nil {
					return nil, err
				}

				return colors, nil
			}

			return nil, fmt.Errorf("The colorscheme file %q was not found.\n\nTo install standard themes, do:\n\ngit clone git@github.com:cointop-sh/colors.git ~/.cointop/colors\n\nFor additional instructions, visit: https://github.com/cointop-sh/colors", path)
		}

		if _, err := toml.DecodeFile(path, &colors); err != nil {
			return nil, err
		}
	}

	return colors, nil
}

func (ct *Cointop) loadAPIChoiceFromConfig() error {
	ct.debuglog("loadAPIKeysFromConfig()")
	apiChoice, ok := ct.config.API.(string)
	if ok {
		apiChoice = strings.TrimSpace(strings.ToLower(apiChoice))
		ct.apiChoice = apiChoice
	}
	return nil
}

func (ct *Cointop) loadFavoritesFromConfig() error {
	ct.debuglog("loadFavoritesFromConfig()")
	for k, arr := range ct.config.Favorites {
		// DEPRECATED: favorites by 'symbol' is deprecated because of collisions. Kept for backward compatibility.
		if k == "symbols" {
			for _, ifc := range arr {
				if v, ok := ifc.(string); ok {
					ct.State.favoritesBySymbol[strings.ToUpper(v)] = true
				}
			}
		} else if k == "names" {
			for _, ifc := range arr {
				if v, ok := ifc.(string); ok {
					ct.State.favorites[v] = true
				}
			}
		}
	}
	return nil
}

func (ct *Cointop) loadPortfolioFromConfig() error {
	ct.debuglog("loadPortfolioFromConfig()")
	for name, holdingsIfc := range ct.config.Portfolio {
		var holdings float64
		var ok bool
		if holdings, ok = holdingsIfc.(float64); !ok {
			if holdingsInt, ok := holdingsIfc.(int64); ok {
				holdings = float64(holdingsInt)
			}
		}

		ct.setPortfolioEntry(name, holdings)
	}
	return nil
}
