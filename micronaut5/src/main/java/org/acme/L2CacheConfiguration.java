package org.acme;

import java.net.URI;

import javax.cache.CacheManager;
import javax.cache.Caching;
import javax.cache.spi.CachingProvider;

import com.github.benmanes.caffeine.jcache.configuration.CaffeineConfiguration;
import com.github.benmanes.caffeine.jcache.spi.CaffeineCachingProvider;

import io.micronaut.context.annotation.Context;
import io.micronaut.context.annotation.Factory;

@Factory
public class L2CacheConfiguration {

    @Context
    public CacheManager jCacheManager() {
        CachingProvider provider = Caching.getCachingProvider(CaffeineCachingProvider.class.getName());
        CacheManager cacheManager = provider.getCacheManager(
                URI.create("caffeine://default"), getClass().getClassLoader());

        createCache(cacheManager, "default");
        createCache(cacheManager, "org.acme.domain.Store");
        createCache(cacheManager, "org.acme.domain.StoreFruitPrice.store");

        return cacheManager;
    }

    private void createCache(CacheManager cacheManager, String cacheName) {
        if (cacheManager.getCache(cacheName) == null) {
            var config = new CaffeineConfiguration<>();
            config.setStoreByValue(false);
            cacheManager.createCache(cacheName, config);
        }
    }
}
